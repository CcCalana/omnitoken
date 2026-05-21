package proxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omnitoken/omnitoken/internal/credentials"
	"github.com/omnitoken/omnitoken/internal/httpx"
)

func TestArkChatProxyRetries429WithNextCredential(t *testing.T) {
	var mu sync.Mutex
	seen := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		mu.Lock()
		seen = append(seen, token)
		mu.Unlock()
		if token == "secret-a" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"quota_owner":"must-not-leak"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"ark-code-latest","choices":[],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(server.Close)

	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: "cred-a", BaseURL: server.URL, Secret: "secret-a", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: "cred-b", BaseURL: server.URL, Secret: "secret-b", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		Timeouts: testTimeouts(),
	}, testLogger(), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[]}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "quota_owner") {
		t.Fatalf("leaked upstream 429 body: %s", rec.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(seen, ",") != "secret-a,secret-b" {
		t.Fatalf("seen credentials = %v", seen)
	}
}

func TestArkChatProxyRetriesStreamBeforeFirstChunk(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[]}\n\n"))
	}))
	t.Cleanup(server.Close)

	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: "cred-a", BaseURL: server.URL, Secret: "secret-a", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: "cred-b", BaseURL: server.URL, Secret: "secret-b", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		Timeouts: testTimeouts(),
	}, testLogger(), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[],"stream":true}`))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "data:") {
		t.Fatalf("unexpected response status=%d body=%s", rec.Code, rec.Body.String())
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
}

func TestArkChatProxyDoesNotRetryPartialFirstRead(t *testing.T) {
	transport := newCountingRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       &partialReadCloser{data: []byte("part\n"), err: io.ErrUnexpectedEOF},
		}, nil
	})
	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: "cred-a", BaseURL: "https://upstream.example", Secret: "secret-a", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: "cred-b", BaseURL: "https://upstream.example", Secret: "secret-b", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		Timeouts: testTimeouts(),
	}, testLogger(), &http.Client{Transport: transport})

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[],"stream":true}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "part\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d", transport.calls)
	}
}

func TestReadWithIdleClosesBodyOnContextDone(t *testing.T) {
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	body := newCloseAwareBlockingReadCloser()
	cancel()

	_, err := readWithIdle(ctx, func() {}, body, make([]byte, 16), time.Second)
	if err == nil {
		t.Fatal("expected context error")
	}
	if !body.closed() {
		t.Fatal("expected body to be closed")
	}

	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+2 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutine count did not return near baseline: before=%d after=%d", before, runtime.NumGoroutine())
}

func TestReadWithIdleClosesBodyWhenContextDoneDuringRead(t *testing.T) {
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	body := newCloseAwareBlockingReadCloser()

	errc := make(chan error, 1)
	go func() {
		_, err := readWithIdle(ctx, func() {}, body, make([]byte, 16), time.Second)
		errc <- err
	}()

	cancel()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("expected context error")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("readWithIdle did not return after context cancel")
	}
	if !body.closed() {
		t.Fatal("expected body to be closed")
	}

	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+2 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutine count did not return near baseline: before=%d after=%d", before, runtime.NumGoroutine())
}

type countingRoundTripper struct {
	fn    func(*http.Request) (*http.Response, error)
	calls int
}

func (r *countingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls++
	return r.fn(req)
}

func newCountingRoundTripper(fn func(*http.Request) (*http.Response, error)) *countingRoundTripper {
	return &countingRoundTripper{fn: fn}
}

type partialReadCloser struct {
	data []byte
	err  error
	read bool
}

func (p *partialReadCloser) Read(buf []byte) (int, error) {
	if p.read {
		return 0, io.EOF
	}
	p.read = true
	copy(buf, p.data)
	return len(p.data), p.err
}

func (p *partialReadCloser) Close() error { return nil }

type closeAwareBlockingReadCloser struct {
	closedCh chan struct{}
	once     sync.Once
}

func newCloseAwareBlockingReadCloser() *closeAwareBlockingReadCloser {
	return &closeAwareBlockingReadCloser{closedCh: make(chan struct{})}
}

func (b *closeAwareBlockingReadCloser) Read([]byte) (int, error) {
	<-b.closedCh
	return 0, io.EOF
}

func (b *closeAwareBlockingReadCloser) Close() error {
	b.once.Do(func() { close(b.closedCh) })
	return nil
}

func (b *closeAwareBlockingReadCloser) closed() bool {
	select {
	case <-b.closedCh:
		return true
	default:
		return false
	}
}

func TestLogCredentialRetryOmitsUpstreamBody(t *testing.T) {
	var out strings.Builder
	logger := slog.New(slog.NewTextHandler(&out, nil))
	handler := NewArkChatProxy(ArkChatConfig{BaseURL: "https://example", APIKey: "key", DefaultModel: testDefaultModel}, logger, nil)
	handler.logCredentialRetry(context.Background(), "cred-a", http.StatusTooManyRequests, CodeUpstream429, 1)
	if strings.Contains(out.String(), "quota_owner") {
		t.Fatalf("log leaked upstream body: %s", out.String())
	}
}
