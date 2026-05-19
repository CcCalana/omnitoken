package anomaly

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestScanWarnsAtThresholdAndSkipsBelow(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	now := time.Date(2026, 5, 19, 12, 5, 0, 0, time.UTC)
	monitor := NewMonitor(&memoryStore{rows: []KeyUsage{
		{APIKeyID: "key-below", APIKeyPrefix: "belowprefix", Count: 99},
		{APIKeyID: "SECRET-api-key-id", APIKeyPrefix: "hitprefix", Count: 100},
	}}, Config{
		Threshold: 100,
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		Now:       func() time.Time { return now },
	})

	if err := monitor.Scan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	logLine := logs.String()
	if !strings.Contains(logLine, "hitprefix") {
		t.Fatalf("expected threshold hit in logs: %s", logLine)
	}
	for _, forbidden := range []string{"belowprefix", "SECRET-api-key-id", "prompt", "user_id", "organization_id", "Authorization"} {
		if strings.Contains(logLine, forbidden) {
			t.Fatalf("log leaked forbidden field %q: %s", forbidden, logLine)
		}
	}
}

func TestScanDebouncesSameKeyWindow(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	now := time.Date(2026, 5, 19, 12, 5, 0, 0, time.UTC)
	monitor := NewMonitor(&memoryStore{rows: []KeyUsage{
		{APIKeyID: "key-1", APIKeyPrefix: "keyprefix", Count: 101},
	}}, Config{
		Threshold: 100,
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		Now:       func() time.Time { return now },
	})

	if err := monitor.Scan(context.Background()); err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if err := monitor.Scan(context.Background()); err != nil {
		t.Fatalf("second scan: %v", err)
	}

	if got := strings.Count(logs.String(), "key anomaly threshold exceeded"); got != 1 {
		t.Fatalf("expected one alert for same window, got %d logs=%s", got, logs.String())
	}
}

func TestScanCleansOldWindowsIncludingUnalertedState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 12, 5, 0, 0, time.UTC)
	monitor := NewMonitor(&memoryStore{}, Config{
		Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		Now:    func() time.Time { return now },
	})
	currentStart, _ := monitor.scanWindow()
	oldStart := currentStart.Add(-DefaultWindow)
	monitor.state[stateKey{apiKeyID: "old-alerted", windowStart: oldStart}] = true
	monitor.state[stateKey{apiKeyID: "old-unalerted", windowStart: oldStart}] = false
	monitor.state[stateKey{apiKeyID: "current", windowStart: currentStart}] = false

	if err := monitor.Scan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if _, exists := monitor.state[stateKey{apiKeyID: "old-alerted", windowStart: oldStart}]; exists {
		t.Fatal("old alerted state was not cleaned")
	}
	if _, exists := monitor.state[stateKey{apiKeyID: "old-unalerted", windowStart: oldStart}]; exists {
		t.Fatal("old unalerted state was not cleaned")
	}
	if _, exists := monitor.state[stateKey{apiKeyID: "current", windowStart: currentStart}]; !exists {
		t.Fatal("current window state should be kept")
	}
}

func TestWindowStartLoggedAsRFC3339UTC(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	cst := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 5, 19, 12, 5, 0, 0, cst)
	monitor := NewMonitor(&memoryStore{rows: []KeyUsage{
		{APIKeyID: "key-1", APIKeyPrefix: "keyprefix", Count: 100},
	}}, Config{
		Threshold: 100,
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		Now:       func() time.Time { return now },
	})

	if err := monitor.Scan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if !strings.Contains(logs.String(), `"window_start":"2026-05-19T04:00:00Z"`) {
		t.Fatalf("window_start should be RFC3339 UTC, got %s", logs.String())
	}
}

func TestScanReturnsStoreError(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("db unavailable")
	monitor := NewMonitor(&memoryStore{err: storeErr}, Config{})

	if err := monitor.Scan(context.Background()); !errors.Is(err, storeErr) {
		t.Fatalf("expected store error, got %v", err)
	}
}

type memoryStore struct {
	rows []KeyUsage
	err  error
}

func (s *memoryStore) ListKeyUsage(_ context.Context, _ time.Time, _ time.Time) ([]KeyUsage, error) {
	return s.rows, s.err
}
