package proxy

import (
	"context"
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
	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/usage"
)

func TestAnthropicToOpenAIRequest(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"chat-balanced",
		"system":[{"type":"text","text":"be terse"},{"type":"image","source":{}}],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"ping"},{"type":"image","source":{}}]},
			{"role":"assistant","content":null}
		],
		"max_tokens":64,
		"stream":true,
		"temperature":0.2,
		"top_p":0.9,
		"stop_sequences":["END"],
		"tools":[{"name":"ignored"}]
	}`)

	got, stream, err := anthropicToOpenAIRequest(body)
	if err != nil {
		t.Fatalf("convert request: %v", err)
	}
	if !stream {
		t.Fatal("expected stream flag")
	}
	var payload map[string]any
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("decode converted request: %v", err)
	}
	if payload["model"] != "chat-balanced" || payload["stop"] == nil || payload["tools"] != nil {
		t.Fatalf("converted payload mismatch: %s", string(got))
	}
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) != 3 {
		t.Fatalf("messages mismatch: %#v", payload["messages"])
	}
	if messages[0].(map[string]any)["role"] != "system" || messages[0].(map[string]any)["content"] != "be terse" {
		t.Fatalf("system message mismatch: %#v", messages[0])
	}
	if messages[1].(map[string]any)["content"] != "ping" {
		t.Fatalf("text block mismatch: %#v", messages[1])
	}
}

func TestAnthropicToOpenAIRequestRejectsEmptyMessages(t *testing.T) {
	t.Parallel()

	_, _, err := anthropicToOpenAIRequest([]byte(`{"model":"m","messages":[]}`))
	if err == nil || !strings.Contains(err.Error(), "messages must not be empty") {
		t.Fatalf("err = %v", err)
	}
}

func TestAnthropicHandlerNonStreamConvertsResponseAndUsageSeesOpenAI(t *testing.T) {
	t.Parallel()

	recorder := &anthropicChannelRecorder{inputs: make(chan usage.RecordInput, 1)}
	openAI := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"messages"`) || strings.Contains(string(body), `"stop_sequences"`) {
			t.Fatalf("unexpected upstream body: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","model":"glm-5.1","choices":[{"index":0,"finish_reason":"length","message":{"role":"assistant","reasoning_content":"think","content":"pong"}}],"usage":{"prompt_tokens":9,"completion_tokens":2,"total_tokens":11,"prompt_tokens_details":{"cached_tokens":3}}}`))
	})
	handler := NewAnthropicMessagesHandler(
		usage.Middleware(recorder, anthropicUsageConfig())(openAI),
		testLogger(),
		AnthropicMessagesConfig{},
	)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newAnthropicRequest(`{"model":"m","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Type       string `json:"type"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens          int `json:"input_tokens"`
			OutputTokens         int `json:"output_tokens"`
			CacheReadInputTokens int `json:"cache_read_input_tokens"`
		} `json:"usage"`
		Content []map[string]string `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode anthropic response: %v", err)
	}
	if payload.Type != "message" || payload.StopReason != "max_tokens" || payload.Usage.InputTokens != 9 || payload.Usage.OutputTokens != 2 || payload.Usage.CacheReadInputTokens != 3 {
		t.Fatalf("anthropic response mismatch: %#v body=%s", payload, rec.Body.String())
	}
	if len(payload.Content) != 2 || payload.Content[0]["type"] != "thinking" || payload.Content[1]["text"] != "pong" {
		t.Fatalf("content mismatch: %#v", payload.Content)
	}

	input := waitAnthropicUsageInput(t, recorder.inputs)
	if !strings.Contains(string(input.Captured), `"choices"`) || strings.Contains(string(input.Captured), `"type":"message"`) {
		t.Fatalf("usage captured non-OpenAI bytes: %s", string(input.Captured))
	}
	if input.ModelRequested != "m" || input.ModelRouted != "m" {
		t.Fatalf("usage metadata mismatch: %#v", input)
	}
}

func TestAnthropicHandlerStreamConvertsSSEAndPreservesFlush(t *testing.T) {
	t.Parallel()

	openAI := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("anthropic transforming writer must preserve http.Flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.TrimPrefix(streamFixture(t), streamFixtureMetaPrefix(t))))
		w.(http.Flusher).Flush()
	})
	handler := NewAnthropicMessagesHandler(openAI, testLogger(), AnthropicMessagesConfig{})

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newAnthropicRequest(`{"model":"m","stream":true,"max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !rec.Flushed {
		t.Fatal("expected flush")
	}
	body := rec.Body.String()
	for _, needle := range []string{"event: message_start", "event: content_block_start", "event: content_block_delta", "event: content_block_stop", "event: message_delta", "event: message_stop"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("stream missing %q: %s", needle, body)
		}
	}
	if !strings.Contains(body, `"output_tokens":2`) {
		t.Fatalf("stream usage mismatch: %s", body)
	}
}

func TestAnthropicStreamIgnoresTrailingDataAfterDone(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	converter := newAnthropicStreamConverter(testLogger())
	err := converter.Write([]byte("data: {\"id\":\"1\",\"model\":\"m\",\"choices\":[{\"delta\":{\"content\":\"pong\"},\"index\":0}]}\n\ndata: [DONE]\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"late\"},\"index\":0}]}\n\n"), &out)
	if err != nil {
		t.Fatalf("convert stream: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "late") {
		t.Fatalf("trailing data leaked after DONE: %s", got)
	}
	if strings.Count(got, "event: message_stop") != 1 {
		t.Fatalf("message_stop count mismatch: %s", got)
	}
}

func TestOpenAIToAnthropicMessageGolden(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "ark", "openai_nonstream_default.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := openAIToAnthropicMessage(stripFixtureMeta(t, body))
	if err != nil {
		t.Fatalf("convert fixture: %v", err)
	}
	if !strings.Contains(string(got), `"type":"message"`) || !strings.Contains(string(got), `"type":"thinking"`) || !strings.Contains(string(got), `"stop_reason":"max_tokens"`) {
		t.Fatalf("unexpected golden conversion: %s", string(got))
	}
}

type anthropicChannelRecorder struct {
	inputs chan usage.RecordInput
}

func (r *anthropicChannelRecorder) Record(_ context.Context, input usage.RecordInput) error {
	r.inputs <- input
	return nil
}

func newAnthropicRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	subject := auth.Subject{UserID: uuid.New(), OrgID: uuid.New(), APIKeyID: uuid.New()}
	ctx := auth.WithSubject(req.Context(), subject)
	ctx = httpx.WithModelRouted(ctx, "m")
	return req.WithContext(ctx)
}

func anthropicUsageConfig() usage.MiddlewareConfig {
	return usage.MiddlewareConfig{
		Provider:      "ark",
		ModelFallback: "ark-code-latest",
		CaptureLimit:  4096,
		RecordTimeout: time.Second,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func waitAnthropicUsageInput(t *testing.T, ch <-chan usage.RecordInput) usage.RecordInput {
	t.Helper()
	select {
	case input := <-ch:
		return input
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for usage record")
	}
	return usage.RecordInput{}
}

func stripFixtureMeta(t *testing.T, body []byte) []byte {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	delete(payload, "_meta")
	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return out
}

func streamFixtureMetaPrefix(t *testing.T) string {
	t.Helper()
	fixture := streamFixture(t)
	index := strings.Index(fixture, "data: ")
	if index < 0 {
		t.Fatal("fixture missing data prefix")
	}
	return fixture[:index]
}
