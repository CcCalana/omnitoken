package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/credentials"
	"github.com/omnitoken/omnitoken/internal/proxy"
	"github.com/omnitoken/omnitoken/internal/quota"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), fakeGatewayStore{}, nil, nil, unavailableChatHandler()).ServeHTTP(rec, req)

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

	newMux(testLogger(), store, nil, nil, unavailableChatHandler()).ServeHTTP(rec, req)

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

	newMux(testLogger(), store, nil, nil, unavailableChatHandler()).ServeHTTP(rec, req)

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

	newMux(testLogger(), validGatewayStore(t), nil, nil, unavailableChatHandler()).ServeHTTP(rec, req)

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

func TestChatCompletionsRejectsExhaustedBudget(t *testing.T) {
	t.Parallel()

	key, store := validGatewayKey(t)
	checker := &fakeBudgetChecker{decision: quota.Decision{
		Allowed:         false,
		Reason:          quota.ReasonMonthlyBudgetExhausted,
		BudgetCents:     sql.NullInt64{Int64: 37, Valid: true},
		UsedBudgetCents: 38,
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	rec := httptest.NewRecorder()

	newMux(testLogger(), store, checker, nil, unavailableChatHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, rec.Code)
	}
	if checker.calls != 1 {
		t.Fatalf("expected quota checker call, got %d", checker.calls)
	}
	var body errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Type != "quota_exceeded" || body.Error.Code != "monthly_budget_exhausted" {
		t.Fatalf("unexpected error envelope: %#v", body)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("37")) || bytes.Contains(rec.Body.Bytes(), []byte("38")) {
		t.Fatalf("quota response leaked budget details: %s", rec.Body.String())
	}
}

func TestChatCompletionsQuotaCheckErrorFailsClosed(t *testing.T) {
	t.Parallel()

	key, store := validGatewayKey(t)
	checker := &fakeBudgetChecker{err: context.Canceled}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	rec := httptest.NewRecorder()

	newMux(testLogger(), store, checker, nil, unavailableChatHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	var body errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "quota_check_failed" {
		t.Fatalf("unexpected error envelope: %#v", body)
	}
}

func TestPostgresURLWithApplicationName(t *testing.T) {
	t.Parallel()

	got := postgresURLWithApplicationName("postgres://user:pass@localhost:5432/db?sslmode=disable", "omnitoken-gateway")
	if !strings.Contains(got, "application_name=omnitoken-gateway") {
		t.Fatalf("expected application_name in URL, got %q", got)
	}
	if !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("expected existing query value to remain, got %q", got)
	}
}

func TestPostgresURLWithApplicationNameKeywordDSN(t *testing.T) {
	t.Parallel()

	got := postgresURLWithApplicationName("host=localhost user=omnitoken dbname=omnitoken sslmode=disable", "omnitoken-gateway")
	if !strings.Contains(got, "application_name=omnitoken-gateway") {
		t.Fatalf("expected application_name in keyword DSN, got %q", got)
	}
}

func TestCredentialPollingReloadsSelector(t *testing.T) {
	t.Parallel()

	store := &fakeCredentialLoader{items: []credentials.Credential{{
		ID:          "fresh",
		Provider:    "deepseek",
		BaseURL:     "https://api.deepseek.com/v1",
		Secret:      "secret",
		Priority:    1,
		Status:      credentials.StatusActive,
		HealthState: credentials.HealthHealthy,
	}}}
	selector := credentials.NewSelector(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startCredentialPolling(ctx, testLogger(), store, selector, time.Millisecond)

	deadline := time.Now().Add(time.Second)
	for {
		item, ok := selector.NextForProvider(context.Background(), "deepseek", nil)
		if ok && item.ID == "fresh" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for credential reload, len=%d", selector.Len())
		}
		time.Sleep(time.Millisecond)
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

type fakeBudgetChecker struct {
	decision quota.Decision
	err      error
	calls    int
}

type fakeCredentialLoader struct {
	items []credentials.Credential
}

func (s *fakeCredentialLoader) Load(context.Context) ([]credentials.Credential, error) {
	return append([]credentials.Credential(nil), s.items...), nil
}

func (c *fakeBudgetChecker) Check(context.Context, auth.Subject, time.Time) (quota.Decision, error) {
	c.calls++
	if c.err != nil {
		return quota.Decision{}, c.err
	}
	return c.decision, nil
}
