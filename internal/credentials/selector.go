package credentials

import (
	"context"
	"sort"
	"sync"
	"time"
)

const DefaultDegradeDuration = 30 * time.Second

type Selector struct {
	mu          sync.Mutex
	clock       func() time.Time
	credentials []Credential
	positions   map[int]int
	degraded    map[string]time.Time
}

func NewSelector(items []Credential) *Selector {
	return NewSelectorWithClock(items, time.Now)
}

func NewSelectorWithClock(items []Credential, clock func() time.Time) *Selector {
	if clock == nil {
		clock = time.Now
	}
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
		if credentials[i].Priority != credentials[j].Priority {
			return credentials[i].Priority < credentials[j].Priority
		}
		return credentials[i].ID < credentials[j].ID
	})
	return &Selector{
		clock:       clock,
		credentials: credentials,
		positions:   map[int]int{},
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

func (s *Selector) Next(_ context.Context, exclude map[string]struct{}) (Credential, bool) {
	if s == nil {
		return Credential{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	priorities := s.prioritiesLocked()
	for _, priority := range priorities {
		pool := s.weightedPoolLocked(priority, now, exclude)
		if len(pool) == 0 {
			continue
		}
		pos := s.positions[priority] % len(pool)
		selected := pool[pos]
		s.positions[priority] = (pos + 1) % len(pool)
		return selected, true
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

func (s *Selector) prioritiesLocked() []int {
	seen := map[int]struct{}{}
	priorities := []int{}
	for _, item := range s.credentials {
		if _, ok := seen[item.Priority]; ok {
			continue
		}
		seen[item.Priority] = struct{}{}
		priorities = append(priorities, item.Priority)
	}
	sort.Ints(priorities)
	return priorities
}

func (s *Selector) weightedPoolLocked(priority int, now time.Time, exclude map[string]struct{}) []Credential {
	pool := []Credential{}
	for _, item := range s.credentials {
		if item.Priority != priority || !eligible(item) {
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

func eligible(item Credential) bool {
	return item.Status == StatusActive && item.HealthState == HealthHealthy
}
