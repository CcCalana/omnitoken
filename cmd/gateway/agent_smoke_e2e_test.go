//go:build e2e

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/proxy"
	"github.com/omnitoken/omnitoken/internal/quota"
	"github.com/omnitoken/omnitoken/internal/usage"
)

func TestAgentSmokeE2E(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("OMNITOKEN_ARK_API_KEY"))
	if apiKey == "" {
		t.Skip("OMNITOKEN_ARK_API_KEY is required for agent smoke e2e")
	}
	maxRequests := e2eMaxRequests(t)
	if maxRequests < 4 {
		t.Skipf("MAX_REQUESTS=%d is below required agent smoke e2e request count 4", maxRequests)
	}

	key, mux, usageStore := newAgentSmokeE2EMux(t, apiKey)

	t.Run("claude-code nonstream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(loadAnthropicFixtureRequest(t, false)))
		req.Header.Set("x-api-key", key.Token)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		var body struct {
			Type  string `json:"type"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Type != "message" || body.Usage.InputTokens+body.Usage.OutputTokens <= 0 {
			t.Fatalf("unexpected Anthropic response: %s", rec.Body.String())
		}
	})

	t.Run("claude-code stream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(loadAnthropicFixtureRequest(t, true)))
		req.Header.Set("x-api-key", key.Token)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		got := rec.Body.String()
		for _, want := range []string{"event: message_start", "event: content_block_delta", "event: message_delta", "event: message_stop"} {
			if !strings.Contains(got, want) {
				t.Fatalf("stream missing %q:\n%s", want, got)
			}
		}
	})

	t.Run("codex nonstream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
			"model":"chat-balanced",
			"messages":[{"role":"user","content":"Output exactly: pong"}],
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
				Message map[string]any `json:"message"`
			} `json:"choices"`
			Usage map[string]any `json:"usage"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(body.Choices) == 0 || body.Choices[0].Message == nil || body.Usage == nil {
			t.Fatalf("unexpected OpenAI response: %s", rec.Body.String())
		}
	})

	t.Run("codex stream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
			"model":"chat-balanced",
			"messages":[{"role":"user","content":"Output exactly: pong"}],
			"stream":true,
			"max_tokens":32
		}`))
		req.Header.Set("Authorization", "Bearer "+key.Token)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		got := rec.Body.String()
		if !strings.Contains(got, "data: ") || !strings.Contains(got, "data: [DONE]") {
			t.Fatalf("stream missing SSE data and DONE:\n%s", got)
		}
	})

	records := usageStore.waitCount(t, 4)
	for _, record := range records {
		if record.ModelRouted == "" || record.Tokens.TotalTokens <= 0 {
			t.Fatalf("usage record missing routed model or tokens: %+v", record)
		}
	}
}

func newAgentSmokeE2EMux(t *testing.T, apiKey string) (auth.PlaintextVirtualKey, http.Handler, *agentSmokeUsageStore) {
	t.Helper()

	key, store := validGatewayKey(t)
	usageStore := newAgentSmokeUsageStore()
	defaultModel := config.Env("OMNITOKEN_ARK_DEFAULT_MODEL", config.DefaultArkModel)
	chatHandler := usage.Middleware(
		usage.NewRecorder(usageStore, testLogger()),
		usage.MiddlewareConfig{Provider: "ark", ModelFallback: defaultModel, Logger: testLogger()},
	)(proxy.NewArkChatProxy(proxy.ArkChatConfig{
		BaseURL:         config.Env("OMNITOKEN_ARK_OPENAI_BASE_URL", config.DefaultArkOpenAIBaseURL),
		APIKey:          apiKey,
		DefaultModel:    defaultModel,
		DisableThinking: true,
		Timeouts: proxy.TimeoutConfig{
			Connect:        5 * time.Second,
			Write:          10 * time.Second,
			FirstByte:      30 * time.Second,
			NonStreamTotal: 60 * time.Second,
			SSEIdle:        30 * time.Second,
		},
	}, testLogger(), nil))

	return key, newMux(
		testLogger(),
		store,
		&fakeBudgetChecker{decision: quota.Decision{Allowed: true}},
		agentSmokeResolver{},
		chatHandler,
	), usageStore
}

func loadAnthropicFixtureRequest(t *testing.T, stream bool) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "ark", "anthropic_nonstream_default.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Meta struct {
			Request map[string]any `json:"request"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	fixture.Meta.Request["stream"] = stream
	encoded, err := json.Marshal(fixture.Meta.Request)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	return string(encoded)
}

func e2eMaxRequests(t *testing.T) int {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv("MAX_REQUESTS"))
	if raw == "" {
		return 10
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("MAX_REQUESTS must be an integer: %v", err)
	}
	return value
}
