package credentials

import (
	"context"
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
