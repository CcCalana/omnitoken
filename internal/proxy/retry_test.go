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
	"sync/atomic"
	"testing"
	"time"

	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/credentials"
	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/usage"
)

const (
	retryCredAID     = "11111111-1111-1111-1111-111111111111"
	retryCredBID     = "22222222-2222-2222-2222-222222222222"
	retryCredCID     = "33333333-3333-3333-3333-333333333333"
	retryDeepSeekAID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	retryDeepSeekBID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	retryArkID       = "cccccccc-cccc-cccc-cccc-cccccccccccc"
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

func TestCrossProviderFallbackAllExcluded(t *testing.T) {
	var deepSeekCalls atomic.Int32
	deepSeek := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		deepSeekCalls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(deepSeek.Close)

	var arkCalls atomic.Int32
	ark := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arkCalls.Add(1)
		if got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); got != "ark-secret" {
			t.Fatalf("authorization secret = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"deepseek-v4-flash","choices":[],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(ark.Close)

	var logs strings.Builder
	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		ModelCatalog: NewStaticModelCatalog([]ProviderModel{{
			Provider:       "ark",
			CanonicalModel: "deepseek-v4-flash",
			ProviderModel:  "deepseek-v4-flash",
		}}),
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: "deepseek-1", Provider: "deepseek", BaseURL: deepSeek.URL, Secret: "deepseek-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: "ark-1", Provider: "ark", BaseURL: ark.URL, Secret: "ark-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		MaxCredentialRetries: 2,
		DegradeDuration:      time.Second,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)

	req := newProxyRequest(t, `{"model":"chat-fast","messages":[]}`)
	ctx := httpx.WithVirtualModel(req.Context(), "chat-fast")
	ctx = httpx.WithModelRouted(ctx, "deepseek-v4-flash")
	ctx = httpx.WithProviderRouted(ctx, "deepseek")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if deepSeekCalls.Load() != 1 || arkCalls.Load() != 1 {
		t.Fatalf("calls deepseek=%d ark=%d", deepSeekCalls.Load(), arkCalls.Load())
	}
	logText := logs.String()
	for _, needle := range []string{
		"cross provider fallback",
		"from_provider=deepseek",
		"to_provider=ark",
		"model_requested=chat-fast",
		"model_routed=deepseek-v4-flash",
		"credential_id=ark-1",
		"reason=all_excluded",
	} {
		if !strings.Contains(logText, needle) {
			t.Fatalf("fallback log missing %q: %s", needle, logText)
		}
	}
}

func TestCrossProviderFallbackModelNotAvailable(t *testing.T) {
	deepSeek := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(deepSeek.Close)

	var arkCalls atomic.Int32
	ark := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		arkCalls.Add(1)
	}))
	t.Cleanup(ark.Close)

	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		ModelCatalog: NewStaticModelCatalog(nil),
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: "deepseek-1", Provider: "deepseek", BaseURL: deepSeek.URL, Secret: "deepseek-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: "ark-1", Provider: "ark", BaseURL: ark.URL, Secret: "ark-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		MaxCredentialRetries: 2,
		DegradeDuration:      time.Second,
		Timeouts:             testTimeouts(),
	}, testLogger(), nil)

	req := newProxyRequest(t, `{"model":"chat-fast","messages":[]}`)
	ctx := httpx.WithVirtualModel(req.Context(), "chat-fast")
	ctx = httpx.WithModelRouted(ctx, "deepseek-v4-flash")
	ctx = httpx.WithProviderRouted(ctx, "deepseek")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeModelNotAvailable)
	if !strings.Contains(rec.Body.String(), "model deepseek-v4-flash is not available on any configured provider") {
		t.Fatalf("missing model availability message: %s", rec.Body.String())
	}
	if arkCalls.Load() != 0 {
		t.Fatalf("ark upstream calls = %d", arkCalls.Load())
	}
}

func TestCrossProviderFallbackAllDegradedReason(t *testing.T) {
	ark := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"deepseek-v4-flash","choices":[],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(ark.Close)

	selector := credentials.NewSelector([]credentials.Credential{
		{ID: "deepseek-1", Provider: "deepseek", BaseURL: "https://deepseek.example", Secret: "deepseek-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		{ID: "ark-1", Provider: "ark", BaseURL: ark.URL, Secret: "ark-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
	})
	selector.MarkDegraded("deepseek-1", time.Minute)

	var logs strings.Builder
	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		ModelCatalog: NewStaticModelCatalog([]ProviderModel{{
			Provider:      "ark",
			ProviderModel: "deepseek-v4-flash",
		}}),
		CredentialSelector:   selector,
		MaxCredentialRetries: 1,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)

	req := newProxyRequest(t, `{"model":"chat-fast","messages":[]}`)
	ctx := httpx.WithVirtualModel(req.Context(), "chat-fast")
	ctx = httpx.WithModelRouted(ctx, "deepseek-v4-flash")
	ctx = httpx.WithProviderRouted(ctx, "deepseek")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "reason=all_degraded") {
		t.Fatalf("fallback log missing all_degraded: %s", logs.String())
	}
}

func TestCrossProviderFallbackPreferredEmptyReason(t *testing.T) {
	ark := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"deepseek-v4-flash","choices":[],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(ark.Close)

	var logs strings.Builder
	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		ModelCatalog: NewStaticModelCatalog([]ProviderModel{{
			Provider:      "ark",
			ProviderModel: "deepseek-v4-flash",
		}}),
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: "ark-1", Provider: "ark", BaseURL: ark.URL, Secret: "ark-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		MaxCredentialRetries: 1,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)

	req := newProxyRequest(t, `{"model":"chat-fast","messages":[]}`)
	ctx := httpx.WithVirtualModel(req.Context(), "chat-fast")
	ctx = httpx.WithModelRouted(ctx, "deepseek-v4-flash")
	ctx = httpx.WithProviderRouted(ctx, "deepseek")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "reason=preferred_empty") {
		t.Fatalf("fallback log missing preferred_empty: %s", logs.String())
	}
}

func TestArkChatProxyRetryConnectionFailureThenRecordsWinningCredential(t *testing.T) {
	failed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("closed upstream should not handle requests")
	}))
	failedURL := failed.URL
	failed.Close()

	var successCalls atomic.Int32
	success := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		successCalls.Add(1)
		if got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); got != "secret-b" {
			t.Fatalf("authorization secret = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"ark-code-latest","choices":[],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(success.Close)

	var logs strings.Builder
	recorder := &retryUsageRecorder{inputs: make(chan usage.RecordInput, 1)}
	proxy := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: retryCredAID, Provider: "ark", BaseURL: failedURL, Secret: "secret-a", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: retryCredBID, Provider: "ark", BaseURL: success.URL, Secret: "secret-b", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		MaxCredentialRetries: 1,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)
	handler := usage.Middleware(recorder, retryUsageConfig())(proxy)

	req := newProxyRequest(t, `{"model":"client-model","messages":[]}`)
	subject := requestSubject(t, req)
	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if successCalls.Load() != 1 {
		t.Fatalf("success calls = %d", successCalls.Load())
	}
	logText := logs.String()
	if !strings.Contains(logText, "upstream credential retry") || !strings.Contains(logText, "credential_id="+retryCredAID) || !strings.Contains(logText, "code="+CodeUpstreamConnectionFailed) {
		t.Fatalf("retry log missing failed credential/code: %s", logText)
	}
	input := waitRetryUsageInput(t, recorder.inputs)
	if input.Subject.APIKeyID != subject.APIKeyID {
		t.Fatalf("api key attribution = %s want %s", input.Subject.APIKeyID, subject.APIKeyID)
	}
	if input.UpstreamCredentialID != retryCredBID {
		t.Fatalf("upstream credential attribution = %q want %q", input.UpstreamCredentialID, retryCredBID)
	}
}

func TestArkChatProxyAllCredentialsExhaustedReturns5xxAndNoUsageRecord(t *testing.T) {
	var callsA, callsB, callsC atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") {
		case "secret-a":
			callsA.Add(1)
		case "secret-b":
			callsB.Add(1)
		case "secret-c":
			callsC.Add(1)
		default:
			t.Fatalf("unexpected authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("vendor body must not leak"))
	}))
	t.Cleanup(server.Close)

	var logs strings.Builder
	recorder := &retryUsageRecorder{inputs: make(chan usage.RecordInput, 1)}
	proxy := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: retryCredAID, Provider: "ark", BaseURL: server.URL, Secret: "secret-a", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: retryCredBID, Provider: "ark", BaseURL: server.URL, Secret: "secret-b", Priority: 2, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: retryCredCID, Provider: "ark", BaseURL: server.URL, Secret: "secret-c", Priority: 3, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		MaxCredentialRetries: 2,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)
	handler := usage.Middleware(recorder, retryUsageConfig())(proxy)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[]}`))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeUpstream5xx)
	if strings.Contains(rec.Body.String(), "vendor body must not leak") {
		t.Fatalf("response leaked upstream body: %s", rec.Body.String())
	}
	if callsA.Load() != 1 || callsB.Load() != 1 || callsC.Load() != 1 {
		t.Fatalf("calls a=%d b=%d c=%d", callsA.Load(), callsB.Load(), callsC.Load())
	}
	logText := logs.String()
	if got := strings.Count(logText, "upstream credential retry"); got != 2 {
		t.Fatalf("retry log count = %d want 2 logs=%s", got, logText)
	}
	for _, credentialID := range []string{retryCredAID, retryCredBID} {
		if !strings.Contains(logText, "credential_id="+credentialID) {
			t.Fatalf("retry log missing credential %s: %s", credentialID, logText)
		}
	}
	assertNoRetryUsageInput(t, recorder.inputs)
}

func TestArkChatProxyCrossProviderFaultFallbackRecordsArkCredential(t *testing.T) {
	failedA := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("closed deepseek upstream should not handle requests")
	}))
	failedAURL := failedA.URL
	failedA.Close()
	failedB := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("closed deepseek upstream should not handle requests")
	}))
	failedBURL := failedB.URL
	failedB.Close()

	var arkCalls atomic.Int32
	ark := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arkCalls.Add(1)
		if got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); got != "ark-secret" {
			t.Fatalf("authorization secret = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"deepseek-v4-flash","choices":[],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(ark.Close)

	var logs strings.Builder
	recorder := &retryUsageRecorder{inputs: make(chan usage.RecordInput, 1)}
	proxy := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		ModelCatalog: NewStaticModelCatalog([]ProviderModel{{
			Provider:      "ark",
			ProviderModel: "deepseek-v4-flash",
		}}),
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: retryDeepSeekAID, Provider: "deepseek", BaseURL: failedAURL, Secret: "deepseek-a", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: retryDeepSeekBID, Provider: "deepseek", BaseURL: failedBURL, Secret: "deepseek-b", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: retryArkID, Provider: "ark", BaseURL: ark.URL, Secret: "ark-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		MaxCredentialRetries: 2,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)
	handler := usage.Middleware(recorder, retryUsageConfig())(proxy)

	req := newProxyRequest(t, `{"model":"chat-fast","messages":[]}`)
	ctx := httpx.WithVirtualModel(req.Context(), "chat-fast")
	ctx = httpx.WithModelRouted(ctx, "deepseek-v4-flash")
	ctx = httpx.WithProviderRouted(ctx, "deepseek")
	req = req.WithContext(ctx)
	subject := requestSubject(t, req)
	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if arkCalls.Load() != 1 {
		t.Fatalf("ark calls = %d", arkCalls.Load())
	}
	logText := logs.String()
	if got := strings.Count(logText, "upstream credential retry"); got != 2 {
		t.Fatalf("retry log count = %d want 2 logs=%s", got, logText)
	}
	for _, needle := range []string{
		"cross provider fallback",
		"from_provider=deepseek",
		"to_provider=ark",
		"credential_id=" + retryArkID,
		"reason=all_excluded",
	} {
		if !strings.Contains(logText, needle) {
			t.Fatalf("fallback log missing %q: %s", needle, logText)
		}
	}
	input := waitRetryUsageInput(t, recorder.inputs)
	if input.Subject.APIKeyID != subject.APIKeyID {
		t.Fatalf("api key attribution = %s want %s", input.Subject.APIKeyID, subject.APIKeyID)
	}
	if input.Provider != "ark" || input.UpstreamCredentialID != retryArkID {
		t.Fatalf("usage attribution provider=%q credential=%q", input.Provider, input.UpstreamCredentialID)
	}
}

func TestArkChatProxyCrossProviderAll5xxExhaustion(t *testing.T) {
	var deepSeekCalls, arkCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") {
		case "deepseek-secret":
			deepSeekCalls.Add(1)
		case "ark-secret":
			arkCalls.Add(1)
		default:
			t.Fatalf("unexpected authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("vendor body must not leak"))
	}))
	t.Cleanup(server.Close)

	var logs strings.Builder
	recorder := &retryUsageRecorder{inputs: make(chan usage.RecordInput, 1)}
	proxy := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		ModelCatalog: NewStaticModelCatalog([]ProviderModel{{
			Provider:      "ark",
			ProviderModel: "deepseek-v4-flash",
		}}),
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: retryDeepSeekAID, Provider: "deepseek", BaseURL: server.URL, Secret: "deepseek-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: retryArkID, Provider: "ark", BaseURL: server.URL, Secret: "ark-secret", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		MaxCredentialRetries: 1,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)
	handler := usage.Middleware(recorder, retryUsageConfig())(proxy)

	req := newProxyRequest(t, `{"model":"chat-fast","messages":[]}`)
	ctx := httpx.WithVirtualModel(req.Context(), "chat-fast")
	ctx = httpx.WithModelRouted(ctx, "deepseek-v4-flash")
	ctx = httpx.WithProviderRouted(ctx, "deepseek")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeUpstream5xx)
	if strings.Contains(rec.Body.String(), "vendor body must not leak") {
		t.Fatalf("response leaked upstream body: %s", rec.Body.String())
	}
	if deepSeekCalls.Load() != 1 || arkCalls.Load() != 1 {
		t.Fatalf("calls deepseek=%d ark=%d", deepSeekCalls.Load(), arkCalls.Load())
	}
	logText := logs.String()
	if got := strings.Count(logText, "upstream credential retry"); got != 1 {
		t.Fatalf("retry log count = %d want 1 logs=%s", got, logText)
	}
	for _, needle := range []string{"cross provider fallback", "reason=all_excluded", "credential_id=" + retryArkID} {
		if !strings.Contains(logText, needle) {
			t.Fatalf("fallback log missing %q: %s", needle, logText)
		}
	}
	assertNoRetryUsageInput(t, recorder.inputs)
}

func TestArkChatProxyDegradeSkipsAndRestoresCredential(t *testing.T) {
	now := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	var callsA, callsB atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") {
		case "secret-a":
			if callsA.Add(1) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
		case "secret-b":
			callsB.Add(1)
		default:
			t.Fatalf("unexpected authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"ark-code-latest","choices":[],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(server.Close)

	selector := credentials.NewSelectorWithClock([]credentials.Credential{
		{ID: retryCredAID, Provider: "ark", BaseURL: server.URL, Secret: "secret-a", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		{ID: retryCredBID, Provider: "ark", BaseURL: server.URL, Secret: "secret-b", Priority: 2, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
	}, func() time.Time { return now })

	if got := selector.AvailabilityForProvider("ark", nil); got.ActiveHealthy != 2 || got.Available != 2 || got.Degraded != 0 {
		t.Fatalf("initial availability = %#v", got)
	}

	var logs strings.Builder
	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel:         testDefaultModel,
		CredentialSelector:   selector,
		MaxCredentialRetries: 1,
		DegradeDuration:      30 * time.Second,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)

	first := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(first, newProxyRequest(t, `{"messages":[]}`))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d body=%s", first.Code, first.Body.String())
	}
	if got := selector.AvailabilityForProvider("ark", nil); got.ActiveHealthy != 2 || got.Available != 1 || got.Degraded != 1 {
		t.Fatalf("degraded availability = %#v", got)
	}

	second := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(second, newProxyRequest(t, `{"messages":[]}`))
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d body=%s", second.Code, second.Body.String())
	}
	if callsA.Load() != 1 || callsB.Load() != 2 {
		t.Fatalf("calls during degrade a=%d b=%d", callsA.Load(), callsB.Load())
	}

	now = now.Add(31 * time.Second)
	if got := selector.AvailabilityForProvider("ark", nil); got.ActiveHealthy != 2 || got.Available != 2 || got.Degraded != 0 {
		t.Fatalf("restored availability = %#v", got)
	}

	third := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(third, newProxyRequest(t, `{"messages":[]}`))
	if third.Code != http.StatusOK {
		t.Fatalf("third status = %d body=%s", third.Code, third.Body.String())
	}
	if callsA.Load() != 2 || callsB.Load() != 2 {
		t.Fatalf("calls after restore a=%d b=%d", callsA.Load(), callsB.Load())
	}
	if !strings.Contains(logs.String(), "code="+CodeUpstream429) {
		t.Fatalf("retry log missing 429 degrade marker: %s", logs.String())
	}
}

func TestArkChatProxyDoesNotRetryAfterSSEChunkThenDisconnect(t *testing.T) {
	var callsA, callsB atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") {
		case "secret-a":
			callsA.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Content-Length", "4096")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"first\"}}]}\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case "secret-b":
			callsB.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"second\"}}]}\n\n"))
		default:
			t.Fatalf("unexpected authorization = %q", r.Header.Get("Authorization"))
		}
	}))
	t.Cleanup(server.Close)

	var logs strings.Builder
	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: retryCredAID, Provider: "ark", BaseURL: server.URL, Secret: "secret-a", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
			{ID: retryCredBID, Provider: "ark", BaseURL: server.URL, Secret: "secret-b", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		}),
		MaxCredentialRetries: 1,
		Timeouts:             testTimeouts(),
	}, slog.New(slog.NewTextHandler(&logs, nil)), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[],"stream":true}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"first"`) {
		t.Fatalf("missing first SSE chunk: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"second"`) {
		t.Fatalf("unexpected retry response body: %s", rec.Body.String())
	}
	if callsA.Load() != 1 || callsB.Load() != 0 {
		t.Fatalf("calls a=%d b=%d", callsA.Load(), callsB.Load())
	}
	if strings.Contains(logs.String(), "upstream credential retry") {
		t.Fatalf("unexpected retry log after streamed chunk: %s", logs.String())
	}
}

func TestArkChatProxyReturnsPoolEmptyWhenSelectorHasNoHealthyCredentials(t *testing.T) {
	handler := NewArkChatProxy(ArkChatConfig{
		DefaultModel: testDefaultModel,
		CredentialSelector: credentials.NewSelector([]credentials.Credential{
			{ID: "disabled", Provider: "ark", BaseURL: "https://ark.example", Secret: "secret", Priority: 1, Status: credentials.StatusDisabled, HealthState: credentials.HealthHealthy},
		}),
		Timeouts: testTimeouts(),
	}, testLogger(), nil)

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, newProxyRequest(t, `{"messages":[]}`))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), CodeUpstreamCredentialPoolEmpty)
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

type retryUsageRecorder struct {
	inputs chan usage.RecordInput
}

func (r *retryUsageRecorder) Record(_ context.Context, input usage.RecordInput) error {
	r.inputs <- input
	return nil
}

func retryUsageConfig() usage.MiddlewareConfig {
	return usage.MiddlewareConfig{
		Provider:      "ark",
		ModelFallback: testDefaultModel,
		CaptureLimit:  4096,
		RecordTimeout: time.Second,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func requestSubject(t *testing.T, req *http.Request) auth.Subject {
	t.Helper()
	subject, ok := auth.SubjectFromContext(req.Context())
	if !ok {
		t.Fatal("request missing auth subject")
	}
	return subject
}

func waitRetryUsageInput(t *testing.T, ch <-chan usage.RecordInput) usage.RecordInput {
	t.Helper()
	select {
	case input := <-ch:
		return input
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for usage record")
	}
	return usage.RecordInput{}
}

func assertNoRetryUsageInput(t *testing.T, ch <-chan usage.RecordInput) {
	t.Helper()
	select {
	case input := <-ch:
		t.Fatalf("unexpected usage record: %#v", input)
	case <-time.After(30 * time.Millisecond):
	}
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
