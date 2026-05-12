package usage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/httpx"
)

func TestMiddlewareRecordsAfterResponseAndPreservesFlush(t *testing.T) {
	recorder := &channelRecorder{inputs: make(chan RecordInput, 1)}
	handler := Middleware(recorder, testMiddlewareConfig())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("usage capture writer must preserve http.Flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"model":"glm-5.1","choices":[],"usage":{"total_tokens":1}}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))

	req := usageRequest(`{"model":"client-model","stream":true}`)
	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !rec.Flushed {
		t.Fatal("expected streaming response to flush")
	}

	input := waitInput(t, recorder.inputs)
	if input.RequestID == "" {
		t.Fatal("missing request id")
	}
	if input.ModelRequested != "client-model" || !input.Streaming {
		t.Fatalf("request metadata mismatch: %#v", input)
	}
	if !strings.Contains(string(input.Captured), `"usage"`) || !strings.Contains(string(input.Captured), "[DONE]") {
		t.Fatalf("captured response mismatch: %s", string(input.Captured))
	}
}

func TestMiddlewareRecorderErrorDoesNotChangeResponse(t *testing.T) {
	recorder := &channelRecorder{
		inputs: make(chan RecordInput, 1),
		err:    errors.New("db down"),
	}
	handler := Middleware(recorder, testMiddlewareConfig())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"glm-5.1","usage":{"total_tokens":1}}`))
	}))

	req := usageRequest(`{"model":"client-model"}`)
	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	_ = waitInput(t, recorder.inputs)
}

func TestMiddlewareSkipsNonSuccessResponse(t *testing.T) {
	recorder := &channelRecorder{inputs: make(chan RecordInput, 1)}
	handler := Middleware(recorder, testMiddlewareConfig())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusBadGateway)
	}))

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, usageRequest(`{"model":"client-model"}`))

	select {
	case input := <-recorder.inputs:
		t.Fatalf("unexpected record input: %#v", input)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestMiddlewareSkipsWhenSubjectMissing(t *testing.T) {
	recorder := &channelRecorder{inputs: make(chan RecordInput, 1)}
	handler := Middleware(recorder, testMiddlewareConfig())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":1}}`))
	}))

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m"}`)))

	select {
	case input := <-recorder.inputs:
		t.Fatalf("unexpected record input: %#v", input)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestMiddlewareNilRecorderPassesThrough(t *testing.T) {
	handler := Middleware(nil, MiddlewareConfig{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
}

type channelRecorder struct {
	inputs chan RecordInput
	err    error
}

func (r *channelRecorder) Record(_ context.Context, input RecordInput) error {
	r.inputs <- input
	return r.err
}

func usageRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	return req.WithContext(auth.WithSubject(req.Context(), testSubject()))
}

func testMiddlewareConfig() MiddlewareConfig {
	return MiddlewareConfig{
		Provider:      "ark",
		ModelFallback: "ark-code-latest",
		CaptureLimit:  1024,
		RecordTimeout: time.Second,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func waitInput(t *testing.T, ch <-chan RecordInput) RecordInput {
	t.Helper()
	select {
	case input := <-ch:
		return input
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for usage record")
	}
	return RecordInput{}
}
