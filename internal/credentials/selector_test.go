package credentials

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSelectorUsesPriorityBeforeWeight(t *testing.T) {
	now := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	selector := NewSelectorWithClock([]Credential{
		{ID: "secondary", Priority: 20, Weight: 10, Status: StatusActive, HealthState: HealthHealthy},
		{ID: "primary-a", Priority: 10, Weight: 2, Status: StatusActive, HealthState: HealthHealthy},
		{ID: "primary-b", Priority: 10, Weight: 1, Status: StatusActive, HealthState: HealthHealthy},
	}, func() time.Time { return now })

	got := []string{}
	for i := 0; i < 4; i++ {
		item, ok := selector.Next(context.Background(), nil)
		if !ok {
			t.Fatal("expected credential")
		}
		got = append(got, item.ID)
	}
	want := []string{"primary-a", "primary-a", "primary-b", "primary-a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sequence = %v want %v", got, want)
		}
	}
}

func TestSelectorSkipsDisabledQuarantinedExcludedAndDegraded(t *testing.T) {
	now := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	selector := NewSelectorWithClock([]Credential{
		{ID: "disabled", Priority: 10, Status: StatusDisabled, HealthState: HealthHealthy},
		{ID: "degraded", Priority: 10, Status: StatusActive, HealthState: HealthDegraded},
		{ID: "quarantined", Priority: 10, Status: StatusActive, HealthState: HealthQuarantined},
		{ID: "healthy", Priority: 10, Status: StatusActive, HealthState: HealthHealthy},
		{ID: "fallback", Priority: 20, Status: StatusActive, HealthState: HealthHealthy},
	}, func() time.Time { return now })
	selector.MarkDegraded("healthy", 30*time.Second)

	item, ok := selector.Next(context.Background(), map[string]struct{}{"fallback": {}})
	if ok {
		t.Fatalf("expected no credential, got %+v", item)
	}

	now = now.Add(31 * time.Second)
	item, ok = selector.Next(context.Background(), nil)
	if !ok || item.ID != "healthy" {
		t.Fatalf("expected healthy after degradation expires, got %+v ok=%t", item, ok)
	}
}

func TestSelectorPrefersProviderThenFallsBack(t *testing.T) {
	now := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	selector := NewSelectorWithClock([]Credential{
		{ID: "ark-a", Provider: "ark", Priority: 1, Status: StatusActive, HealthState: HealthHealthy},
		{ID: "deepseek-a", Provider: "deepseek", Priority: 2, Status: StatusActive, HealthState: HealthHealthy},
		{ID: "deepseek-b", Provider: "deepseek", Priority: 2, Status: StatusActive, HealthState: HealthHealthy},
	}, func() time.Time { return now })

	item, ok := selector.NextForProvider(context.Background(), "deepseek", nil)
	if !ok || item.Provider != "deepseek" {
		t.Fatalf("expected preferred deepseek credential, got %+v ok=%t", item, ok)
	}

	exclude := map[string]struct{}{"deepseek-a": {}, "deepseek-b": {}}
	item, ok = selector.NextForProvider(context.Background(), "deepseek", exclude)
	if !ok || item.ID != "ark-a" {
		t.Fatalf("expected fallback ark credential, got %+v ok=%t", item, ok)
	}
}

func TestSelectorReportsProviderAvailability(t *testing.T) {
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	selector := NewSelectorWithClock([]Credential{
		{ID: "excluded", Provider: "deepseek", Priority: 1, Status: StatusActive, HealthState: HealthHealthy},
		{ID: "temporary-degraded", Provider: "deepseek", Priority: 1, Status: StatusActive, HealthState: HealthHealthy},
		{ID: "disabled", Provider: "deepseek", Priority: 1, Status: StatusDisabled, HealthState: HealthHealthy},
		{ID: "health-degraded", Provider: "deepseek", Priority: 1, Status: StatusActive, HealthState: HealthDegraded},
		{ID: "ark", Provider: "ark", Priority: 1, Status: StatusActive, HealthState: HealthHealthy},
	}, func() time.Time { return now })
	selector.MarkDegraded("temporary-degraded", time.Minute)

	got := selector.AvailabilityForProvider("deepseek", map[string]struct{}{"excluded": {}})
	want := ProviderAvailability{ActiveHealthy: 2, Excluded: 1, Degraded: 1, Available: 0}
	if got != want {
		t.Fatalf("availability = %+v want %+v", got, want)
	}

	now = now.Add(time.Minute + time.Second)
	got = selector.AvailabilityForProvider("deepseek", map[string]struct{}{"excluded": {}})
	want = ProviderAvailability{ActiveHealthy: 2, Excluded: 1, Available: 1}
	if got != want {
		t.Fatalf("availability after expiry = %+v want %+v", got, want)
	}
}

func TestSelectorHandlesNilAndDefaults(t *testing.T) {
	var nilSelector *Selector
	if nilSelector.Len() != 0 {
		t.Fatal("nil selector length should be zero")
	}
	if item, ok := nilSelector.Next(context.Background(), nil); ok {
		t.Fatalf("nil selector returned credential: %+v", item)
	}

	selector := NewSelectorWithClock([]Credential{{ID: "defaulted", Priority: 1}}, nil)
	if selector.Len() != 1 {
		t.Fatalf("selector length = %d", selector.Len())
	}
	item, ok := selector.Next(context.Background(), nil)
	if !ok || item.ID != "defaulted" || item.Weight != 1 || item.Status != StatusActive || item.HealthState != HealthHealthy {
		t.Fatalf("defaulted credential = %+v ok=%t", item, ok)
	}
	selector.MarkDegraded("", time.Second)
}

func TestSelectorReplaceSwapsPoolAtomically(t *testing.T) {
	now := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	selector := NewSelectorWithClock([]Credential{
		{ID: "old", Provider: "ark", Priority: 1, Status: StatusActive, HealthState: HealthHealthy},
	}, func() time.Time { return now })

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = selector.NextForProvider(context.Background(), "deepseek", nil)
			}
		}()
	}
	delta := selector.Replace([]Credential{
		{ID: "new", Provider: "deepseek", Priority: 1, Status: StatusActive, HealthState: HealthHealthy},
	})
	wg.Wait()

	if delta != 0 {
		t.Fatalf("replace delta = %d want 0", delta)
	}
	item, ok := selector.NextForProvider(context.Background(), "deepseek", nil)
	if !ok || item.ID != "new" {
		t.Fatalf("expected new credential after replace, got %+v ok=%t", item, ok)
	}
}
