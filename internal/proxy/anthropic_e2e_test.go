//go:build e2e

package proxy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/httpx"
)

func TestAnthropicMessagesHandlerE2E(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("OMNITOKEN_ARK_API_KEY"))
	if apiKey == "" {
		t.Skip("OMNITOKEN_ARK_API_KEY is required for e2e")
	}

	baseURL := config.Env("OMNITOKEN_ARK_OPENAI_BASE_URL", config.DefaultArkOpenAIBaseURL)
	defaultModel := config.Env("OMNITOKEN_ARK_DEFAULT_MODEL", config.DefaultArkModel)
	openAI := NewArkChatProxy(ArkChatConfig{
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
	handler := NewAnthropicMessagesHandler(openAI, slog.New(slog.NewTextHandler(io.Discard, nil)), AnthropicMessagesConfig{})

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newAnthropicE2ERequest(t))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Type  string `json:"type"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Type != "message" || payload.Usage.InputTokens+payload.Usage.OutputTokens <= 0 {
		t.Fatalf("unexpected anthropic response: %s", rec.Body.String())
	}
}

func newAnthropicE2ERequest(t *testing.T) *http.Request {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "ark", "anthropic_nonstream_default.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Meta struct {
			Request json.RawMessage `json:"request"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(fixture.Meta.Request)))
	subject := auth.Subject{UserID: uuid.New(), OrgID: uuid.New(), APIKeyID: uuid.New()}
	return req.WithContext(auth.WithSubject(req.Context(), subject))
}
