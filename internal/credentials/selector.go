package credentials

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultDegradeDuration = 30 * time.Second

type Selector struct {
	mu          sync.Mutex
	clock       func() time.Time
	credentials []Credential
	positions   map[string]int
	degraded    map[string]time.Time
}

type ProviderAvailability struct {
	ActiveHealthy int
	Excluded      int
	Degraded      int
	Available     int
}

func NewSelector(items []Credential) *Selector {
	return NewSelectorWithClock(items, time.Now)
}

func NewSelectorWithClock(items []Credential, clock func() time.Time) *Selector {
	if clock == nil {
		clock = time.Now
	}
	return &Selector{
		clock:       clock,
		credentials: normalizeCredentials(items),
		positions:   map[string]int{},
		degraded:    map[string]time.Time{},
	}
}

func (s *Selector) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.credentials)
}

func (s *Selector) Replace(items []Credential) int {
	if s == nil {
		return 0
	}
	credentials := normalizeCredentials(items)
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := len(s.credentials)
	s.credentials = credentials
	s.positions = map[string]int{}
	s.degraded = pruneDegraded(s.degraded, credentials)
	return len(credentials) - previous
}

func (s *Selector) Next(ctx context.Context, exclude map[string]struct{}) (Credential, bool) {
	return s.NextForProvider(ctx, "", exclude)
}

func (s *Selector) NextForProvider(_ context.Context, preferredProvider string, exclude map[string]struct{}) (Credential, bool) {
	if s == nil {
		return Credential{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	providers := s.providerOrderLocked(preferredProvider)
	for _, provider := range providers {
		priorities := s.prioritiesLocked(provider)
		for _, priority := range priorities {
			pool := s.weightedPoolLocked(provider, priority, now, exclude)
			if len(pool) == 0 {
				continue
			}
			key := positionKey(provider, priority)
			pos := s.positions[key] % len(pool)
			selected := pool[pos]
			s.positions[key] = (pos + 1) % len(pool)
			return selected, true
		}
	}
	return Credential{}, false
}

func (s *Selector) MarkDegraded(id string, duration time.Duration) {
	if s == nil || id == "" {
		return
	}
	if duration <= 0 {
		duration = DefaultDegradeDuration
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.degraded[id] = s.clock().Add(duration)
}

func (s *Selector) AvailabilityForProvider(provider string, exclude map[string]struct{}) ProviderAvailability {
	if s == nil {
		return ProviderAvailability{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	provider = normalizedProvider(provider)
	var status ProviderAvailability
	for _, item := range s.credentials {
		if normalizedProvider(item.Provider) != provider || !eligible(item) {
			continue
		}
		status.ActiveHealthy++
		if _, skip := exclude[item.ID]; skip {
			status.Excluded++
			continue
		}
		if until, degraded := s.degraded[item.ID]; degraded && now.Before(until) {
			status.Degraded++
			continue
		}
		status.Available++
	}
	return status
}

func (s *Selector) providerOrderLocked(preferredProvider string) []string {
	preferredProvider = strings.TrimSpace(preferredProvider)
	seen := map[string]struct{}{}
	providers := []string{}
	if preferredProvider != "" {
		providers = append(providers, preferredProvider)
		seen[preferredProvider] = struct{}{}
	}
	for _, item := range s.credentials {
		provider := strings.TrimSpace(item.Provider)
		if provider == "" {
			provider = "ark"
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		seen[provider] = struct{}{}
		providers = append(providers, provider)
	}
	return providers
}

func (s *Selector) prioritiesLocked(provider string) []int {
	seen := map[int]struct{}{}
	priorities := []int{}
	for _, item := range s.credentials {
		if normalizedProvider(item.Provider) != provider {
			continue
		}
		if _, ok := seen[item.Priority]; ok {
			continue
		}
		seen[item.Priority] = struct{}{}
		priorities = append(priorities, item.Priority)
	}
	sort.Ints(priorities)
	return priorities
}

func (s *Selector) weightedPoolLocked(provider string, priority int, now time.Time, exclude map[string]struct{}) []Credential {
	pool := []Credential{}
	for _, item := range s.credentials {
		if normalizedProvider(item.Provider) != provider || item.Priority != priority || !eligible(item) {
			continue
		}
		if _, skip := exclude[item.ID]; skip {
			continue
		}
		if until, degraded := s.degraded[item.ID]; degraded && now.Before(until) {
			continue
		}
		weight := item.Weight
		if weight <= 0 {
			weight = 1
		}
		for i := 0; i < weight; i++ {
			pool = append(pool, item)
		}
	}
	return pool
}

func positionKey(provider string, priority int) string {
	return provider + ":" + strconv.Itoa(priority)
}

func normalizedProvider(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "ark"
	}
	return provider
}

func eligible(item Credential) bool {
	return item.Status == StatusActive && item.HealthState == HealthHealthy
}

func normalizeCredentials(items []Credential) []Credential {
	credentials := append([]Credential(nil), items...)
	for i := range credentials {
		if credentials[i].Weight <= 0 {
			credentials[i].Weight = 1
		}
		if credentials[i].Status == "" {
			credentials[i].Status = StatusActive
		}
		if credentials[i].HealthState == "" {
			credentials[i].HealthState = HealthHealthy
		}
	}
	sort.SliceStable(credentials, func(i, j int) bool {
		if credentials[i].Provider != credentials[j].Provider {
			return credentials[i].Provider < credentials[j].Provider
		}
		if credentials[i].Priority != credentials[j].Priority {
			return credentials[i].Priority < credentials[j].Priority
		}
		return credentials[i].ID < credentials[j].ID
	})
	return credentials
}

func pruneDegraded(degraded map[string]time.Time, credentials []Credential) map[string]time.Time {
	if len(degraded) == 0 {
		return map[string]time.Time{}
	}
	keep := make(map[string]struct{}, len(credentials))
	for _, item := range credentials {
		keep[item.ID] = struct{}{}
	}
	out := map[string]time.Time{}
	for id, until := range degraded {
		if _, ok := keep[id]; ok {
			out[id] = until
		}
	}
	return out
}
