package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/proxy"
	"github.com/omnitoken/omnitoken/internal/quota"
	"github.com/omnitoken/omnitoken/internal/router"
	"github.com/omnitoken/omnitoken/internal/usage"
)

func TestAgentSmokeClaudeCodeXAPIKey(t *testing.T) {
	t.Parallel()

	key, mux, store := newAgentSmokeMux(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"chat-balanced",
		"max_tokens":32,
		"messages":[{"role":"user","content":"ping"}]
	}`))
	req.Header.Set("x-api-key", key.Token)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Type  string `json:"type"`
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Type != "message" || body.Usage.InputTokens <= 0 {
		t.Fatalf("unexpected anthropic response: %s", rec.Body.String())
	}
	record := store.wait(t)
	if record.ModelRequested != "chat-balanced" || record.ModelRouted != "mock-real" || record.Provider != "ark" {
		t.Fatalf("unexpected usage record: %+v", record)
	}
}

func TestAgentSmokeCodexBearer(t *testing.T) {
	t.Parallel()

	key, mux, store := newAgentSmokeMux(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"chat-balanced",
		"messages":[{"role":"user","content":"ping"}],
		"max_tokens":32
	}`))
	req.Header.Set("Authorization", "Bearer "+key.Token)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Choices) == 0 || body.Choices[0].Message.Content == "" || body.Usage == nil {
		t.Fatalf("unexpected OpenAI response: %s", rec.Body.String())
	}
	record := store.wait(t)
	if record.ModelRequested != "chat-balanced" || record.ModelRouted != "mock-real" || record.Provider != "ark" {
		t.Fatalf("unexpected usage record: %+v", record)
	}
}

func TestAgentSmokeInvalidXAPIKeyUsesAnthropicError(t *testing.T) {
	t.Parallel()

	_, mux, _ := newAgentSmokeMux(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"chat-balanced",
		"max_tokens":32,
		"messages":[{"role":"user","content":"ping"}]
	}`))
	req.Header.Set("x-api-key", "omt_invalid")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Type  string `json:"type"`
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Type != "error" || body.Error.Type != "authentication_error" {
		t.Fatalf("unexpected Anthropic error envelope: %s", rec.Body.String())
	}
}

func TestAgentSmokeInvalidBearerUsesOpenAIError(t *testing.T) {
	t.Parallel()

	_, mux, _ := newAgentSmokeMux(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"chat-balanced",
		"messages":[{"role":"user","content":"ping"}]
	}`))
	req.Header.Set("Authorization", "Bearer omt_invalid")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Type != "authentication_error" || body.Error.Code != "invalid_api_key" {
		t.Fatalf("unexpected OpenAI error envelope: %#v", body)
	}
}

func TestAgentSmokeMissingAuthHeaders(t *testing.T) {
	t.Parallel()

	_, mux, _ := newAgentSmokeMux(t)
	for _, tc := range []struct {
		name     string
		path     string
		body     string
		wantJSON string
	}{
		{
			name:     "claude-code",
			path:     "/v1/messages",
			body:     `{"model":"chat-balanced","max_tokens":32,"messages":[{"role":"user","content":"ping"}]}`,
			wantJSON: `"type":"error"`,
		},
		{
			name:     "codex",
			path:     "/v1/chat/completions",
			body:     `{"model":"chat-balanced","messages":[{"role":"user","content":"ping"}]}`,
			wantJSON: `"code":"invalid_api_key"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(compactJSON(rec.Body.String()), tc.wantJSON) {
				t.Fatalf("missing expected error format %s: %s", tc.wantJSON, rec.Body.String())
			}
		})
	}
}

func newAgentSmokeMux(t *testing.T) (auth.PlaintextVirtualKey, http.Handler, *agentSmokeUsageStore) {
	t.Helper()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer upstream-secret" {
			t.Fatalf("unexpected upstream authorization header: %q", r.Header.Get("Authorization"))
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if strings.Contains(readBody(t, r), `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(`data: {"id":"chatcmpl-smoke","model":"mock-real","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":""}]}` + "\n\n"))
			_, _ = w.Write([]byte(`data: {"id":"chatcmpl-smoke","model":"mock-real","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":""}]}` + "\n\n"))
			_, _ = w.Write([]byte(`data: {"id":"chatcmpl-smoke","model":"mock-real","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n"))
			_, _ = w.Write([]byte(`data: {"id":"chatcmpl-smoke","model":"mock-real","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}` + "\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-smoke","object":"chat.completion","created":1,"model":"mock-real","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`))
	}))
	t.Cleanup(upstream.Close)

	key, store := validGatewayKey(t)
	usageStore := newAgentSmokeUsageStore()
	chatHandler := usage.Middleware(
		usage.NewRecorder(usageStore, testLogger()),
		usage.MiddlewareConfig{Provider: "ark", ModelFallback: "mock-real", Logger: testLogger()},
	)(proxy.NewArkChatProxy(proxy.ArkChatConfig{
		BaseURL:         upstream.URL,
		APIKey:          "upstream-secret",
		DefaultModel:    "mock-real",
		DisableThinking: true,
	}, testLogger(), nil))

	mux := newMux(
		testLogger(),
		store,
		&fakeBudgetChecker{decision: quota.Decision{Allowed: true}},
		agentSmokeResolver{},
		chatHandler,
	)
	return key, mux, usageStore
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read upstream request: %v", err)
	}
	return string(body)
}

func compactJSON(value string) string {
	return strings.Join(strings.Fields(value), "")
}

type agentSmokeResolver struct{}

func (agentSmokeResolver) Resolve(_ context.Context, requested string) (router.Resolution, error) {
	return router.Resolution{RealModel: "mock-real", Provider: "ark", IsVirtual: requested == "chat-balanced"}, nil
}

type agentSmokeUsageStore struct {
	mu      sync.Mutex
	records []usage.UsageRecord
	ready   chan struct{}
	changed chan struct{}
}

func newAgentSmokeUsageStore() *agentSmokeUsageStore {
	return &agentSmokeUsageStore{
		ready:   make(chan struct{}),
		changed: make(chan struct{}, 1),
	}
}

func (s *agentSmokeUsageStore) InsertUsage(_ context.Context, record usage.UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	if len(s.records) == 1 {
		close(s.ready)
	}
	select {
	case s.changed <- struct{}{}:
	default:
	}
	return nil
}

func (s *agentSmokeUsageStore) wait(t *testing.T) usage.UsageRecord {
	t.Helper()
	select {
	case <-s.ready:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for usage record")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.records[0]
}

func (s *agentSmokeUsageStore) recordsSnapshot() []usage.UsageRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]usage.UsageRecord(nil), s.records...)
}

func (s *agentSmokeUsageStore) waitCount(t *testing.T, want int) []usage.UsageRecord {
	t.Helper()

	deadline := time.After(5 * time.Second)
	for {
		records := s.recordsSnapshot()
		if len(records) >= want {
			return records
		}
		select {
		case <-s.changed:
		case <-deadline:
			t.Fatalf("timed out waiting for %d usage records, got %d", want, len(records))
		}
	}
}
