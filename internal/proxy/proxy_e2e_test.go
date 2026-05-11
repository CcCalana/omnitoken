//go:build e2e

package proxy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/httpx"
)

func TestArkChatProxyE2E(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("OMNITOKEN_ARK_API_KEY"))
	if apiKey == "" {
		t.Skip("OMNITOKEN_ARK_API_KEY is required for e2e")
	}

	baseURL := config.Env("OMNITOKEN_ARK_OPENAI_BASE_URL", config.DefaultArkOpenAIBaseURL)
	defaultModel := config.Env("OMNITOKEN_ARK_DEFAULT_MODEL", config.DefaultArkModel)
	handler := NewArkChatProxy(ArkChatConfig{
		BaseURL:         baseURL,
		APIKey:          apiKey,
		DefaultModel:    defaultModel,
		DisableThinking: true,
		Timeouts: TimeoutConfig{
			Connect:        5 * time.Second,
			Write:          10 * time.Second,
			FirstByte:      30 * time.Second,
			NonStreamTotal: 60 * time.Second,
			SSEIdle:        30 * time.Second,
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	t.Run("nonstream includes usage", func(t *testing.T) {
		body := `{"model":"client-model","messages":[{"role":"user","content":"Output exactly: pong"}],"max_tokens":32}`
		rec := httptest.NewRecorder()
		httpx.RequestID(handler).ServeHTTP(rec, newE2ERequest(body))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload["usage"] == nil {
			t.Fatalf("expected usage in response: %s", rec.Body.String())
		}
	})

	t.Run("stream includes final usage chunk", func(t *testing.T) {
		body := `{"model":"client-model","messages":[{"role":"user","content":"Output exactly: pong"}],"stream":true,"max_tokens":32}`
		rec := httptest.NewRecorder()
		httpx.RequestID(handler).ServeHTTP(rec, newE2ERequest(body))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		got := rec.Body.String()
		if !strings.Contains(got, `"choices":[]`) || !strings.Contains(got, `"usage":{`) || !strings.Contains(got, "data: [DONE]") {
			t.Fatalf("expected final usage chunk and DONE event: %s", got)
		}
	})
}

func newE2ERequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	subject := auth.Subject{
		UserID:   uuid.New(),
		OrgID:    uuid.New(),
		APIKeyID: uuid.New(),
	}
	return req.WithContext(auth.WithSubject(req.Context(), subject))
}
