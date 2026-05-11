package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/proxy"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), fakeGatewayStore{}, unavailableChatHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body.Status != "ok" || body.Service != "gateway" {
		t.Fatalf("unexpected health body: %+v", body)
	}
}

func TestModels(t *testing.T) {
	t.Parallel()

	key, store := validGatewayKey(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	rec := httptest.NewRecorder()

	newMux(testLogger(), store, unavailableChatHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body modelResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode model response: %v", err)
	}
	if body.Object != "list" {
		t.Fatalf("expected list object, got %q", body.Object)
	}
	if len(body.Data) == 0 {
		t.Fatal("expected at least one model")
	}
}

func TestChatCompletionsRequiresConfiguredArk(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	key, store := validGatewayKey(t)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	req.Header.Set("X-Request-Id", "req-chat")
	rec := httptest.NewRecorder()

	newMux(testLogger(), store, unavailableChatHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("expected generated request id header")
	}
	if rec.Header().Get("X-Request-Id") == "req-chat" {
		t.Fatal("client request id should not overwrite internal request id")
	}
	if rec.Header().Get("X-Upstream-Request-Id") != "req-chat" {
		t.Fatalf("upstream request id header = %q", rec.Header().Get("X-Upstream-Request-Id"))
	}

	var body errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "upstream_not_configured" {
		t.Fatalf("unexpected error envelope: %#v", body)
	}
}

func TestModelsRequiresVirtualKey(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), validGatewayStore(t), unavailableChatHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var body errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Type != "authentication_error" || body.Error.Code != "invalid_api_key" {
		t.Fatalf("unexpected error envelope: %#v", body)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func unavailableChatHandler() http.Handler {
	return proxy.NewArkChatProxy(proxy.ArkChatConfig{
		BaseURL:      "http://127.0.0.1:1",
		APIKey:       "",
		DefaultModel: "ark-code-latest",
	}, testLogger(), nil)
}

func validGatewayStore(t *testing.T) fakeGatewayStore {
	t.Helper()

	_, store := validGatewayKey(t)
	return store
}

func validGatewayKey(t *testing.T) (auth.PlaintextVirtualKey, fakeGatewayStore) {
	t.Helper()

	key, err := auth.GenerateVirtualKeyFromReader(bytes.NewReader(bytes.Repeat([]byte{0x68}, 39)))
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key, fakeGatewayStore{
		record: auth.VirtualKeyRecord{
			APIKeyID: uuid.New(),
			OrgID:    uuid.New(),
			UserID:   uuid.New(),
			KeyHash:  key.Hash,
			Status:   "active",
		},
	}
}

type fakeGatewayStore struct {
	record auth.VirtualKeyRecord
	err    error
}

func (s fakeGatewayStore) LookupVirtualKey(context.Context, string) (auth.VirtualKeyRecord, error) {
	if s.err != nil {
		return auth.VirtualKeyRecord{}, s.err
	}
	return s.record, nil
}
