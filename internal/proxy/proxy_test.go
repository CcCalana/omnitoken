package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/httpx"
)

const (
	testArkKey       = "test-ark-key"
	testDefaultModel = "ark-code-latest"
)

func TestArkChatProxyScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		requestBody   string
		apiKey        string
		handler       func(t *testing.T) http.Handler
		baseURL       func(*httptest.Server) string
		wantStatus    int
		wantCode      string
		wantContains  []string
		wantUpstream  bool
		wantStream    bool
		wantFlushed   bool
		wantNoBody    string
		wantNoHit     bool
		firstByteWait time.Duration
		sseIdle       time.Duration
	}{
		{
			name:        "nonstream",
			requestBody: `{"model":"client-model","messages":[{"role":"user","content":"hi"}]}`,
			apiKey:      testArkKey,
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assertUpstreamRequest(t, r, false)
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Server", "mock-ark")
					w.Header().Set("X-Powered-By", "mock")
					w.Header().Set("Set-Cookie", "session=bad")
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}],"usage":{"total_tokens":11}}`))
				})
			},
			wantStatus:   http.StatusCreated,
			wantContains: []string{`"usage":{"total_tokens":11}`, `"pong"`},
			wantUpstream: true,
		},
		{
			name:        "stream",
			requestBody: `{"model":"client-model","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"keep":"me"}}`,
			apiKey:      testArkKey,
			handler: func(t *testing.T) http.Handler {
				fixture := streamFixture(t)
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assertUpstreamRequest(t, r, true)
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
					_, _ = w.Write([]byte(fixture))
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
				})
			},
			wantStatus:   http.StatusOK,
			wantContains: []string{`"choices":[]`, `"usage":{"completion_tokens":2`, "data: [DONE]"},
			wantUpstream: true,
			wantStream:   true,
			wantFlushed:  true,
		},
		{
			name:        "upstream timeout",
			requestBody: `{"model":"client-model","messages":[{"role":"user","content":"hi"}]}`,
			apiKey:      testArkKey,
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assertUpstreamRequest(t, r, false)
					time.Sleep(80 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				})
			},
			wantStatus:    http.StatusBadGateway,
			wantCode:      CodeUpstreamTimeout,
			wantUpstream:  true,
			firstByteWait: 20 * time.Millisecond,
		},
		{
			name:        "upstream 502",
			requestBody: `{"model":"client-model","messages":[{"role":"user","content":"hi"}]}`,
			apiKey:      testArkKey,
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assertUpstreamRequest(t, r, false)
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte("vendor body must not leak"))
				})
			},
			wantStatus:   http.StatusBadGateway,
			wantCode:     CodeUpstream5xx,
			wantUpstream: true,
			wantNoBody:   "vendor body must not leak",
		},
		{
			name:        "connection failed",
			requestBody: `{"model":"client-model","messages":[{"role":"user","content":"hi"}]}`,
			apiKey:      testArkKey,
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					t.Fatal("upstream should not receive a request")
				})
			},
			baseURL: func(server *httptest.Server) string {
				server.Close()
				return server.URL
			},
			wantStatus: http.StatusBadGateway,
			wantCode:   CodeUpstreamConnectionFailed,
			wantNoHit:  true,
		},
		{
			name:        "missing api key",
			requestBody: `{"model":"client-model","messages":[{"role":"user","content":"hi"}]}`,
			apiKey:      "",
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					t.Fatal("upstream should not receive a request")
				})
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   CodeUpstreamNotConfigured,
			wantNoHit:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var hit atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hit.Store(true)
				tt.handler(t).ServeHTTP(w, r)
			}))
			t.Cleanup(server.Close)

			baseURL := server.URL
			if tt.baseURL != nil {
				baseURL = tt.baseURL(server)
			}

			timeouts := TimeoutConfig{
				Connect:        50 * time.Millisecond,
				Write:          50 * time.Millisecond,
				FirstByte:      200 * time.Millisecond,
				NonStreamTotal: 500 * time.Millisecond,
				SSEIdle:        200 * time.Millisecond,
			}
			if tt.firstByteWait > 0 {
				timeouts.FirstByte = tt.firstByteWait
			}
			if tt.sseIdle > 0 {
				timeouts.SSEIdle = tt.sseIdle
			}

			handler := NewArkChatProxy(ArkChatConfig{
				BaseURL:         baseURL,
				APIKey:          tt.apiKey,
				DefaultModel:    testDefaultModel,
				DisableThinking: true,
				MaxRequestBytes: DefaultMaxRequestBytes,
				Timeouts:        timeouts,
			}, testLogger(), nil)

			req := newProxyRequest(t, tt.requestBody)
			rec := httptest.NewRecorder()

			httpx.RequestID(handler).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if tt.wantUpstream && !hit.Load() {
				t.Fatal("expected upstream request")
			}
			if tt.wantNoHit && hit.Load() {
				t.Fatal("unexpected upstream request")
			}
			if tt.wantCode != "" {
				assertErrorCode(t, rec.Body.Bytes(), tt.wantCode)
			}
			for _, needle := range tt.wantContains {
				if !strings.Contains(rec.Body.String(), needle) {
					t.Fatalf("response missing %q: %s", needle, rec.Body.String())
				}
			}
			if tt.wantNoBody != "" && strings.Contains(rec.Body.String(), tt.wantNoBody) {
				t.Fatalf("response leaked upstream body: %s", rec.Body.String())
			}
			if tt.wantFlushed && !rec.Flushed {
				t.Fatal("expected streaming response to flush")
			}
			if rec.Header().Get("Server") != "" || rec.Header().Get("X-Powered-By") != "" || rec.Header().Get("Set-Cookie") != "" {
				t.Fatalf("unsafe upstream headers leaked: %#v", rec.Header())
			}
		})
	}
}

func TestArkChatProxyInvalidRequestBody(t *testing.T) {
	t.Parallel()

	handler := NewArkChatProxy(ArkChatConfig{
		BaseURL:      "http://127.0.0.1:1",
		APIKey:       testArkKey,
		DefaultModel: testDefaultModel,
		Timeouts:     testTimeouts(),
	}, testLogger(), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeInvalidRequest)
}

func TestArkChatProxyRejectsOversizeRequestBody(t *testing.T) {
	t.Parallel()

	handler := NewArkChatProxy(ArkChatConfig{
		BaseURL:         "http://127.0.0.1:1",
		APIKey:          testArkKey,
		DefaultModel:    testDefaultModel,
		MaxRequestBytes: 16,
		Timeouts:        testTimeouts(),
	}, testLogger(), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, strings.Repeat("a", 64)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeInvalidRequest)
}

func TestArkChatProxyRejectsInvalidStreamContentType(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertUpstreamRequest(t, r, true)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"unexpected":true}`))
	}))
	t.Cleanup(server.Close)

	handler := NewArkChatProxy(ArkChatConfig{
		BaseURL:         server.URL,
		APIKey:          testArkKey,
		DefaultModel:    testDefaultModel,
		DisableThinking: true,
		Timeouts:        testTimeouts(),
	}, testLogger(), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[],"stream":true,"stream_options":{"keep":"me"}}`))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeUpstreamInvalidResponse)
}

func TestArkChatProxyStreamIdleTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertUpstreamRequest(t, r, true)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(80 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	handler := NewArkChatProxy(ArkChatConfig{
		BaseURL:         server.URL,
		APIKey:          testArkKey,
		DefaultModel:    testDefaultModel,
		DisableThinking: true,
		Timeouts: TimeoutConfig{
			Connect:        50 * time.Millisecond,
			Write:          50 * time.Millisecond,
			FirstByte:      200 * time.Millisecond,
			NonStreamTotal: 500 * time.Millisecond,
			SSEIdle:        20 * time.Millisecond,
		},
	}, testLogger(), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[],"stream":true,"stream_options":{"keep":"me"}}`))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeUpstreamTimeout)
}

func TestArkChatProxyMethodNotAllowed(t *testing.T) {
	t.Parallel()

	handler := NewArkChatProxy(ArkChatConfig{
		BaseURL:      "http://127.0.0.1:1",
		APIKey:       testArkKey,
		DefaultModel: testDefaultModel,
		Timeouts:     testTimeouts(),
	}, testLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeInvalidRequest)
}

func TestArkChatProxyReplacesNonObjectStreamOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		streamOptions, ok := body["stream_options"].(map[string]any)
		if !ok || streamOptions["include_usage"] != true {
			t.Fatalf("stream_options = %#v", body["stream_options"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(server.Close)

	handler := NewArkChatProxy(ArkChatConfig{
		BaseURL:         server.URL,
		APIKey:          testArkKey,
		DefaultModel:    testDefaultModel,
		DisableThinking: true,
		Timeouts:        testTimeouts(),
	}, testLogger(), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[],"stream":true,"stream_options":"bad"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChatCompletionsURL(t *testing.T) {
	t.Parallel()

	got, err := chatCompletionsURL("https://ark.example.com/api/coding/v3/")
	if err != nil {
		t.Fatalf("chat URL: %v", err)
	}
	if got != "https://ark.example.com/api/coding/v3/chat/completions" {
		t.Fatalf("url = %q", got)
	}
	if _, err := chatCompletionsURL("://bad"); err == nil {
		t.Fatal("expected invalid URL error")
	}
}

func TestClassifyTimeoutErrors(t *testing.T) {
	t.Parallel()

	timeoutErr := &net.DNSError{IsTimeout: true}
	if got := classifyUpstreamRequestError(timeoutErr); got != CodeUpstreamTimeout {
		t.Fatalf("timeout classification = %q", got)
	}
	if got := classifyUpstreamRequestError(context.Canceled); got != CodeUpstreamConnectionFailed {
		t.Fatalf("connection classification = %q", got)
	}
}

func TestCopyBufferedResponseReadError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	status, code, err := copyBufferedResponse(rec, &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       errReadCloser{},
	})
	if err == nil {
		t.Fatal("expected read error")
	}
	if status != http.StatusBadGateway || code != CodeUpstreamInvalidResponse {
		t.Fatalf("status/code = %d/%s", status, code)
	}
}

func TestReadWithIdleContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n, err := readWithIdle(ctx, func() {}, blockingReadCloser{}, make([]byte, 8), time.Second)
	if n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("read = %d, %v", n, err)
	}
}

func TestClassifyStreamingReadError(t *testing.T) {
	t.Parallel()

	if got := classifyStreamingReadError(io.EOF); got != "" {
		t.Fatalf("EOF classification = %q", got)
	}
	if got := classifyStreamingReadError(context.DeadlineExceeded); got != CodeUpstreamTimeout {
		t.Fatalf("timeout classification = %q", got)
	}
	if got := classifyStreamingReadError(io.ErrUnexpectedEOF); got != CodeUpstreamInvalidResponse {
		t.Fatalf("invalid classification = %q", got)
	}
}

func TestTimeoutConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := TimeoutConfig{}.withDefaults()
	if cfg.Connect == 0 || cfg.Write == 0 || cfg.FirstByte == 0 || cfg.NonStreamTotal == 0 || cfg.SSEIdle == 0 {
		t.Fatalf("defaults not applied: %#v", cfg)
	}
}

func TestArkChatProxyUsesRoutedModelAndProviderCompatibility(t *testing.T) {
	t.Parallel()

	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"deepseek-v4-flash","choices":[],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(server.Close)

	handler := NewArkChatProxy(ArkChatConfig{
		BaseURL:         server.URL,
		APIKey:          testArkKey,
		DefaultModel:    testDefaultModel,
		DisableThinking: true,
		Timeouts:        testTimeouts(),
	}, testLogger(), nil)
	req := newProxyRequest(t, `{"model":"chat-fast","messages":[]}`)
	ctx := httpx.WithModelRouted(req.Context(), "deepseek-v4-flash")
	ctx = httpx.WithProviderRouted(ctx, "deepseek")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if captured["model"] != "deepseek-v4-flash" {
		t.Fatalf("model = %#v", captured["model"])
	}
	if _, ok := captured["thinking"]; ok {
		t.Fatalf("deepseek request should not include thinking: %#v", captured)
	}
}

func TestIsEventStreamFallback(t *testing.T) {
	t.Parallel()

	if !isEventStream("text/event-stream; charset=utf-8") {
		t.Fatal("expected event stream")
	}
	if isEventStream("application/json") {
		t.Fatal("unexpected event stream")
	}
}

func assertUpstreamRequest(t *testing.T, r *http.Request, wantStream bool) {
	t.Helper()

	if r.Method != http.MethodPost {
		t.Fatalf("method = %s", r.Method)
	}
	if r.URL.Path != "/chat/completions" {
		t.Fatalf("path = %q", r.URL.Path)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+testArkKey {
		t.Fatalf("authorization = %q", got)
	}
	if strings.Contains(r.Header.Get("Authorization"), "test-virtual-key") {
		t.Fatal("client virtual key leaked upstream")
	}
	if got := r.Header.Get(httpx.RequestIDHeader); got == "" {
		t.Fatal("missing upstream request id")
	}
	if got := r.Header.Get("Cookie"); got != "" {
		t.Fatalf("cookie leaked upstream: %q", got)
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode upstream body: %v", err)
	}
	if body["model"] != testDefaultModel {
		t.Fatalf("model = %#v", body["model"])
	}
	thinking, ok := body["thinking"].(map[string]any)
	if !ok || thinking["type"] != "disabled" {
		t.Fatalf("thinking = %#v", body["thinking"])
	}
	if got, _ := body["stream"].(bool); got != wantStream {
		t.Fatalf("stream = %t want %t", got, wantStream)
	}
	streamOptions, ok := body["stream_options"].(map[string]any)
	if wantStream {
		if !ok || streamOptions["include_usage"] != true {
			t.Fatalf("stream_options = %#v", body["stream_options"])
		}
		if streamOptions["keep"] != "me" {
			t.Fatalf("stream_options did not preserve fields: %#v", streamOptions)
		}
		return
	}
	if ok {
		t.Fatalf("nonstream request unexpectedly had stream_options: %#v", streamOptions)
	}
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()

	var got errorEnvelope
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode error envelope: %v body=%s", err, string(body))
	}
	if got.Error.Code != want {
		t.Fatalf("code = %q want %q body=%s", got.Error.Code, want, string(body))
	}
}

func newProxyRequest(t *testing.T, body string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-virtual-key")
	req.Header.Set("Cookie", "session=client")
	subject := auth.Subject{
		UserID:   uuid.New(),
		OrgID:    uuid.New(),
		APIKeyID: uuid.New(),
	}
	return req.WithContext(auth.WithSubject(req.Context(), subject))
}

func streamFixture(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "golden", "ark", "openai_stream_no_thinking_with_usage.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stream fixture: %v", err)
	}

	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "_meta:") {
			continue
		}
		if strings.TrimSpace(line) == "" && len(lines) == 0 {
			continue
		}
		lines = append(lines, line)
	}
	fixture := strings.TrimLeft(strings.Join(lines, "\n"), "\n")
	if !strings.Contains(fixture, `"choices":[]`) || !strings.Contains(fixture, `"usage":{`) {
		t.Fatalf("stream fixture missing final usage chunk")
	}
	return fixture
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testTimeouts() TimeoutConfig {
	return TimeoutConfig{
		Connect:        50 * time.Millisecond,
		Write:          50 * time.Millisecond,
		FirstByte:      200 * time.Millisecond,
		NonStreamTotal: 500 * time.Millisecond,
		SSEIdle:        200 * time.Millisecond,
	}
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func (errReadCloser) Close() error {
	return nil
}

type blockingReadCloser struct{}

func (blockingReadCloser) Read([]byte) (int, error) {
	select {}
}

func (blockingReadCloser) Close() error {
	return nil
}
