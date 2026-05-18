package audit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omnitoken/omnitoken/internal/httpx"
)

func TestMiddlewareRecordsWriteRequest(t *testing.T) {
	t.Parallel()

	recorder := newChannelRecorder()
	handler := Middleware(recorder, MiddlewareConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC) },
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetAction(r.Context(), "create_virtual_key")
		SetResource(r.Context(), "virtual_key", "key-1")
		SetBefore(r.Context(), nil)
		SetAfter(r.Context(), map[string]string{"key_prefix": "abcdefghijkl"})
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(`{}`))
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("User-Agent", "audit-test")
	rec := httptest.NewRecorder()

	httpx.RequestID(handler).ServeHTTP(rec, req)

	entry := recorder.wait(t)
	if entry.Actor.ID != "bootstrap" || entry.Actor.Type != ActorTypeBootstrap {
		t.Fatalf("unexpected actor: %+v", entry.Actor)
	}
	if entry.Action != "create_virtual_key" || entry.ResourceType != "virtual_key" || entry.ResourceID != "key-1" {
		t.Fatalf("unexpected action/resource: %+v", entry)
	}
	if entry.StatusCode != http.StatusCreated || entry.UserAgent != "audit-test" {
		t.Fatalf("unexpected status/user agent: %+v", entry)
	}
	if !entry.IP.Equal(net.ParseIP("192.0.2.10")) || entry.RequestID == "" {
		t.Fatalf("unexpected ip/request id: ip=%v request_id=%q", entry.IP, entry.RequestID)
	}
}

func TestMiddlewareSkipsReadRequest(t *testing.T) {
	t.Parallel()

	recorder := newChannelRecorder()
	handler := Middleware(recorder, MiddlewareConfig{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil))
	recorder.expectNoRecord(t)
}

func TestMiddlewareRecordsFailedWriteStatuses(t *testing.T) {
	t.Parallel()

	for _, statusCode := range []int{http.StatusUnprocessableEntity, http.StatusInternalServerError} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			t.Parallel()

			recorder := newChannelRecorder()
			handler := Middleware(recorder, MiddlewareConfig{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(statusCode)
			}))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/api/admin/example", nil))

			entry := recorder.wait(t)
			if entry.StatusCode != statusCode {
				t.Fatalf("expected status %d, got %+v", statusCode, entry)
			}
		})
	}
}

func TestMiddlewareFailureIncrementsMetricAndDoesNotBlock(t *testing.T) {
	t.Parallel()

	beforeMetric := AuditRecordFailuresTotal.Value()
	recorder := newChannelRecorder()
	recorder.err = errors.New("audit db unavailable")
	handler := Middleware(recorder, MiddlewareConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", nil))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected business response to pass, got %d", rec.Code)
	}
	recorder.wait(t)
	deadline := time.Now().Add(time.Second)
	for AuditRecordFailuresTotal.Value() <= beforeMetric {
		if time.Now().After(deadline) {
			t.Fatalf("expected audit failure metric to increment from %d, got %d", beforeMetric, AuditRecordFailuresTotal.Value())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMiddlewareRecordsPanicThenRethrows(t *testing.T) {
	t.Parallel()

	recorder := newChannelRecorder()
	handler := Middleware(recorder, MiddlewareConfig{})(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	}))

	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("expected panic to be rethrown")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", nil))
	}()

	entry := recorder.wait(t)
	if entry.Action != ActionPanicRecovered || entry.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected panic audit entry: %+v", entry)
	}
}

type channelRecorder struct {
	entries chan Entry
	err     error
}

func newChannelRecorder() *channelRecorder {
	return &channelRecorder{entries: make(chan Entry, 1)}
}

func (r *channelRecorder) Record(_ context.Context, entry Entry) error {
	r.entries <- entry
	return r.err
}

func (r *channelRecorder) wait(t *testing.T) Entry {
	t.Helper()
	select {
	case entry := <-r.entries:
		return entry
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for audit entry")
		return Entry{}
	}
}

func (r *channelRecorder) expectNoRecord(t *testing.T) {
	t.Helper()
	select {
	case entry := <-r.entries:
		t.Fatalf("unexpected audit entry: %+v", entry)
	case <-time.After(50 * time.Millisecond):
	}
}
