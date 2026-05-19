package anomaly

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	DefaultThreshold = 100
	DefaultWindow    = 5 * time.Minute
	DefaultInterval  = 5 * time.Minute
)

type KeyUsage struct {
	APIKeyID     string
	APIKeyPrefix string
	Count        int64
}

type Store interface {
	ListKeyUsage(context.Context, time.Time, time.Time) ([]KeyUsage, error)
}

type Config struct {
	Threshold int
	Window    time.Duration
	Interval  time.Duration
	Logger    *slog.Logger
	Now       func() time.Time
}

type Monitor struct {
	store     Store
	threshold int64
	window    time.Duration
	interval  time.Duration
	logger    *slog.Logger
	now       func() time.Time

	mu    sync.Mutex
	state map[stateKey]bool
}

type stateKey struct {
	apiKeyID    string
	windowStart time.Time
}

func NewMonitor(store Store, cfg Config) *Monitor {
	cfg = cfg.withDefaults()
	return &Monitor{
		store:     store,
		threshold: int64(cfg.Threshold),
		window:    cfg.Window,
		interval:  cfg.Interval,
		logger:    cfg.Logger,
		now:       cfg.Now,
		state:     make(map[stateKey]bool),
	}
}

func (m *Monitor) Start(ctx context.Context) {
	if m == nil || m.store == nil {
		return
	}
	go m.run(ctx)
}

func (m *Monitor) Scan(ctx context.Context) error {
	if m == nil || m.store == nil {
		return nil
	}
	windowStart, windowEnd := m.scanWindow()
	rows, err := m.store.ListKeyUsage(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("list key usage anomalies: %w", err)
	}

	m.cleanupBefore(windowStart)
	for _, row := range rows {
		m.processRow(row, windowStart)
	}
	return nil
}

func (m *Monitor) run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.Scan(ctx); err != nil {
				m.logger.Warn("key anomaly scan failed", "err", err)
			}
		}
	}
}

func (m *Monitor) processRow(row KeyUsage, windowStart time.Time) {
	apiKeyID := strings.TrimSpace(row.APIKeyID)
	if apiKeyID == "" {
		return
	}
	key := stateKey{apiKeyID: apiKeyID, windowStart: windowStart}
	if row.Count < m.threshold {
		m.markSeen(key)
		return
	}
	if !m.markAlerted(key) {
		return
	}

	m.logger.Warn("key anomaly threshold exceeded",
		"api_key_prefix", defaultString(row.APIKeyPrefix, "unknown"),
		"window_start", windowStart.UTC().Format(time.RFC3339),
		"count", row.Count,
		"threshold", m.threshold,
	)
}

func (m *Monitor) scanWindow() (time.Time, time.Time) {
	windowEnd := m.now().UTC().Truncate(time.Second)
	windowStart := windowEnd.Add(-m.window)
	return windowStart, windowEnd
}

func (m *Monitor) markSeen(key stateKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.state[key]; !exists {
		m.state[key] = false
	}
}

func (m *Monitor) markAlerted(key stateKey) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state[key] {
		return false
	}
	m.state[key] = true
	return true
}

func (m *Monitor) cleanupBefore(windowStart time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.state {
		if key.windowStart.Before(windowStart) {
			delete(m.state, key)
		}
	}
}

func (cfg Config) withDefaults() Config {
	if cfg.Threshold <= 0 {
		cfg.Threshold = DefaultThreshold
	}
	if cfg.Window <= 0 {
		cfg.Window = DefaultWindow
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return cfg
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
