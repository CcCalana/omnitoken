package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/audit"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/credentials"
	"github.com/omnitoken/omnitoken/internal/rbac"
	usageanomaly "github.com/omnitoken/omnitoken/internal/usage/anomaly"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body.Status != "ok" || body.Service != "admin" {
		t.Fatalf("unexpected health body: %+v", body)
	}
}

func TestAdminReadRoutesRequireBootstrapToken(t *testing.T) {
	t.Parallel()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	paths := []string{
		"/api/admin/overview",
		"/api/admin/users",
		"/api/admin/models",
		"/api/admin/audit-logs",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			newMux(testLogger(), cfg, nil, nil, nil).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
			}
			assertErrorCode(t, rec, "invalid_api_key")
		})
	}
}

func TestAdminAuthMiddlewareTokenCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		configToken   string
		requestHeader string
		wantStatus    int
		wantCode      string
	}{
		{
			name:          "correct token",
			configToken:   "dev-bootstrap",
			requestHeader: "Bearer dev-bootstrap",
			wantStatus:    http.StatusOK,
		},
		{
			name:          "wrong token",
			configToken:   "dev-bootstrap",
			requestHeader: "Bearer wrong-token",
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "invalid_api_key",
		},
		{
			name:          "empty bearer token",
			configToken:   "dev-bootstrap",
			requestHeader: "Bearer ",
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "invalid_api_key",
		},
		{
			name:          "unconfigured bootstrap token",
			configToken:   "",
			requestHeader: "Bearer dev-bootstrap",
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "invalid_api_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := testAdminConfig()
			cfg.BootstrapToken = tt.configToken
			req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
			if tt.requestHeader != "" {
				req.Header.Set("Authorization", tt.requestHeader)
			}
			rec := httptest.NewRecorder()

			newMux(testLogger(), cfg, nil, nil, nil).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if tt.wantCode != "" {
				assertErrorCode(t, rec, tt.wantCode)
			}
		})
	}
}

func TestOverviewFallsBackToZeroWithoutStore(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	rec := httptest.NewRecorder()

	makeOverviewHandler(testLogger(), nil, func() time.Time { return now }).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"trend":[]`) || !strings.Contains(rec.Body.String(), `"model_usage":[]`) {
		t.Fatalf("expected empty JSON arrays, got %s", rec.Body.String())
	}

	var body overviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode overview response: %v", err)
	}
	if body.Period != "2026-05" {
		t.Fatalf("unexpected period: %q", body.Period)
	}
	if body.TotalTokens != 0 || body.EstimatedCostUSD != 0 || body.ActiveUsers != 0 || body.QuotaWarningUsers != 0 {
		t.Fatalf("expected zero overview, got %+v", body)
	}
	if len(body.Trend) != 0 || len(body.ModelUsage) != 0 {
		t.Fatalf("expected empty overview series, got %+v", body)
	}
}

func TestOverviewReturnsStoreData(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	store := &fakeOverviewStore{
		response: overviewResponse{
			Period:            "2026-05",
			TotalTokens:       126,
			EstimatedCostUSD:  0.003,
			ActiveUsers:       2,
			QuotaWarningUsers: 0,
			Trend: []dailyTokenUsage{
				{Date: "2026-05-11", Tokens: 126, Cost: 0.003},
			},
			ModelUsage: []modelUsage{
				{Model: "ark-code-latest", Tokens: 126, Cost: 0.003, Share: 1},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	rec := httptest.NewRecorder()

	makeOverviewHandler(testLogger(), store, func() time.Time { return now }).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !store.called || !store.now.Equal(now) {
		t.Fatalf("store was not called with fixed now: called=%v now=%s", store.called, store.now)
	}

	var body overviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode overview response: %v", err)
	}
	if body.TotalTokens != 126 || body.EstimatedCostUSD != 0.003 || body.ActiveUsers != 2 {
		t.Fatalf("unexpected overview response: %+v", body)
	}
	if len(body.ModelUsage) != 1 || body.ModelUsage[0].Share != 1 {
		t.Fatalf("unexpected model usage: %+v", body.ModelUsage)
	}
}

func TestOverviewStoreErrorReturns500(t *testing.T) {
	t.Parallel()

	store := &fakeOverviewStore{err: errors.New("database unavailable")}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	rec := httptest.NewRecorder()

	makeOverviewHandler(testLogger(), store, func() time.Time {
		return time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	var body map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"]["code"] != "overview_query_failed" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestUsersFallsBackToEmptyWithoutStore(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	newMux(testLogger(), cfg, nil, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"users":[]`) {
		t.Fatalf("expected empty users array, got %s", rec.Body.String())
	}
}

func TestUsersReturnsStoreData(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	store := &fakeOverviewStore{
		users: usersResponse{
			Users: []adminUserUsage{
				{
					UserID:          "00000000-0000-0000-0000-000000000201",
					Email:           "admin@democorp.local",
					DisplayName:     "Demo Admin",
					UsedTokens:      42,
					UsedBudgetCents: 2,
					BudgetCents:     int64Ptr(100),
					Quota:           100,
					Status:          "active",
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	makeUsersHandler(testLogger(), store, func() time.Time { return now }).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !store.usersCalled || !store.usersNow.Equal(now) {
		t.Fatalf("store was not called with fixed now: called=%v now=%s", store.usersCalled, store.usersNow)
	}
	var body usersResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode users response: %v", err)
	}
	if len(body.Users) != 1 || body.Users[0].UsedTokens != 42 || body.Users[0].Quota != 100 ||
		body.Users[0].BudgetCents == nil || *body.Users[0].BudgetCents != 100 || body.Users[0].UsedBudgetCents != 2 {
		t.Fatalf("unexpected users response: %+v", body)
	}
}

func TestUsersStoreErrorReturns500(t *testing.T) {
	t.Parallel()

	store := &fakeOverviewStore{usersErr: errors.New("database unavailable")}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	makeUsersHandler(testLogger(), store, time.Now).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	var body map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"]["code"] != "users_query_failed" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestUpdateUserQuotaCreatesAuditRecord(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	beforeBudget := int64(100)
	afterBudget := int64(50)
	store := &fakeOverviewStore{
		updateResult: updateUserBudgetResult{
			BeforeBudgetCents: &beforeBudget,
			AfterBudgetCents:  &afterBudget,
		},
	}
	recorder := newAdminAuditRecorder()
	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userID.String()+"/quota", strings.NewReader(`{"budget_cents":50}`))
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, store, nil, nil, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !store.updateCalled || store.updateUserID != userID || store.updateBudgetCents == nil || *store.updateBudgetCents != 50 {
		t.Fatalf("unexpected update call: called=%v user=%s budget=%v", store.updateCalled, store.updateUserID, store.updateBudgetCents)
	}
	var body updateUserQuotaResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if body.UserID != userID.String() || body.BudgetCents == nil || *body.BudgetCents != 50 {
		t.Fatalf("unexpected update response: %+v", body)
	}

	entry := recorder.wait(t)
	if entry.Action != "update_quota" || entry.ResourceType != "user_quota" || entry.ResourceID != userID.String() {
		t.Fatalf("unexpected audit resource: %+v", entry)
	}
	before := entry.Before.(map[string]any)
	after := entry.After.(map[string]any)
	if got := before["budget_cents"].(*int64); *got != 100 {
		t.Fatalf("unexpected before snapshot: %#v", before)
	}
	if got := after["budget_cents"].(*int64); *got != 50 {
		t.Fatalf("unexpected after snapshot: %#v", after)
	}
}

func TestUpdateUserQuotaAllowsClearingBudget(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	beforeBudget := int64(100)
	store := &fakeOverviewStore{
		updateResult: updateUserBudgetResult{
			BeforeBudgetCents: &beforeBudget,
			AfterBudgetCents:  nil,
		},
	}
	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userID.String()+"/quota", strings.NewReader(`{"budget_cents":null}`))
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, store, nil, nil, audit.NewRecorder(nil, testLogger())).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if store.updateBudgetCents != nil {
		t.Fatalf("expected nil budget update, got %v", store.updateBudgetCents)
	}
	var body updateUserQuotaResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if body.BudgetCents != nil {
		t.Fatalf("expected nil budget response, got %+v", body)
	}
}

func TestUpdateUserQuotaRejectsInvalidBudget(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recorder := newAdminAuditRecorder()
	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userID.String()+"/quota", strings.NewReader(`{"budget_cents":-1}`))
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, &fakeOverviewStore{}, nil, nil, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	entry := recorder.wait(t)
	if entry.Action != "update_quota" || entry.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected failed audit entry: %+v", entry)
	}
}

func TestModelsFallsBackToEmptyWithoutStore(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/models", nil)
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	newMux(testLogger(), cfg, nil, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"models":[]`) {
		t.Fatalf("expected empty models array, got %s", rec.Body.String())
	}
}

func TestModelsReturnsStoreData(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	store := &fakeOverviewStore{
		models: modelsResponse{
			Models: []adminModelUsage{
				{
					Model:            "glm-5.1",
					Provider:         "ark",
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
					CostUSD:          0.0001,
					CallCount:        2,
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/models", nil)
	rec := httptest.NewRecorder()

	makeModelsHandler(testLogger(), store, func() time.Time { return now }).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !store.modelsCalled || !store.modelsNow.Equal(now) {
		t.Fatalf("store was not called with fixed now: called=%v now=%s", store.modelsCalled, store.modelsNow)
	}
	var body modelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode models response: %v", err)
	}
	if len(body.Models) != 1 || body.Models[0].Model != "glm-5.1" || body.Models[0].CallCount != 2 {
		t.Fatalf("unexpected models response: %+v", body)
	}
}

func TestModelsStoreErrorReturns500(t *testing.T) {
	t.Parallel()

	store := &fakeOverviewStore{modelsErr: errors.New("database unavailable")}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/models", nil)
	rec := httptest.NewRecorder()

	makeModelsHandler(testLogger(), store, time.Now).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	var body map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"]["code"] != "models_query_failed" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestCredentialsHandlerDoesNotExposeSecrets(t *testing.T) {
	t.Parallel()

	store := &fakeOverviewStore{
		credentials: credentialsResponse{Credentials: []credentials.PublicCredential{
			{
				ID:          "00000000-0000-0000-0000-000000000301",
				Provider:    "ark",
				BaseURL:     "https://ark.example/v3",
				Priority:    10,
				Weight:      1,
				Status:      "active",
				HealthState: "healthy",
			},
		}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/credentials", nil)
	rec := httptest.NewRecorder()

	makeCredentialsHandler(testLogger(), store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if strings.Contains(rec.Body.String(), "encrypted_secret") || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("credentials response leaked secret fields: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"provider":"ark"`) {
		t.Fatalf("missing credential metadata: %s", rec.Body.String())
	}
}

func TestCredentialsAliasRoute(t *testing.T) {
	t.Parallel()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodGet, "/admin/credentials", nil)
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, &fakeOverviewStore{}, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestAuditLogsReturnsEmptyWithoutStore(t *testing.T) {
	t.Parallel()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs", nil)
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, nil, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var body []adminAuditLog
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode audit logs response: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty audit log array, got %+v", body)
	}
}

func TestAuditLogsHandlerParsesFilters(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	until := since.Add(time.Hour)
	store := &fakeOverviewStore{
		auditLogs: []adminAuditLog{{
			ActorID:      "bootstrap",
			ActorType:    "bootstrap",
			Action:       "create_virtual_key",
			ResourceType: "virtual_key",
			StatusCode:   http.StatusCreated,
			CreatedAt:    since.Format(time.RFC3339),
		}},
	}
	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodGet,
		"/api/admin/audit-logs?actor_id=bootstrap&resource_type=virtual_key&resource_id=key-1&since="+since.Format(time.RFC3339)+"&until="+until.Format(time.RFC3339)+"&limit=700",
		nil,
	)
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, store, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !store.auditLogsCalled {
		t.Fatal("expected store to be called")
	}
	if store.auditLogFilters.ActorID != "bootstrap" ||
		store.auditLogFilters.ResourceType != "virtual_key" ||
		store.auditLogFilters.ResourceID != "key-1" ||
		store.auditLogFilters.Limit != 500 {
		t.Fatalf("unexpected filters: %+v", store.auditLogFilters)
	}
	if store.auditLogFilters.Since == nil || !store.auditLogFilters.Since.Equal(since) {
		t.Fatalf("unexpected since filter: %+v", store.auditLogFilters.Since)
	}
	if store.auditLogFilters.Until == nil || !store.auditLogFilters.Until.Equal(until) {
		t.Fatalf("unexpected until filter: %+v", store.auditLogFilters.Until)
	}
	var body []adminAuditLog
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode audit logs response: %v", err)
	}
	if len(body) != 1 || body[0].Action != "create_virtual_key" {
		t.Fatalf("unexpected audit logs response: %+v", body)
	}
}

func TestAuditLogsHandlerRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?since=bad-time", nil)
	rec := httptest.NewRecorder()

	makeAuditLogsHandler(testLogger(), &fakeOverviewStore{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	assertErrorCode(t, rec, "invalid_audit_log_filters")
}

func TestAuditLogsStoreErrorReturns500(t *testing.T) {
	t.Parallel()

	store := &fakeOverviewStore{auditLogsErr: errors.New("database unavailable")}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs", nil)
	rec := httptest.NewRecorder()

	makeAuditLogsHandler(testLogger(), store).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	assertErrorCode(t, rec, "audit_logs_query_failed")
}

func TestPostgresOverviewStoreLoadOverviewMapsSQLResults(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{"total_tokens", "estimated_cost_usd", "active_users"},
			rows: [][]driver.Value{
				{int64(300), float64(1.25), int64(2)},
			},
		},
		{
			columns: []string{"usage_date", "total_tokens", "cost_usd"},
			rows: [][]driver.Value{
				{"2026-05-11", int64(180), float64(0.75)},
				{"2026-05-12", int64(120), float64(0.50)},
			},
		},
		{
			columns: []string{"model", "total_tokens", "cost_usd"},
			rows: [][]driver.Value{
				{"ark-code-latest", int64(200), float64(0.80)},
				{"ark-lite", int64(100), float64(0.45)},
			},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadOverview(context.Background(), now)
	if err != nil {
		t.Fatalf("load overview: %v", err)
	}

	if got.Period != "2026-05" || got.TotalTokens != 300 || got.EstimatedCostUSD != 1.25 || got.ActiveUsers != 2 {
		t.Fatalf("unexpected overview summary: %+v", got)
	}
	if len(got.Trend) != 2 || got.Trend[0].Date != "2026-05-11" || got.Trend[1].Tokens != 120 {
		t.Fatalf("unexpected trend: %+v", got.Trend)
	}
	if len(got.ModelUsage) != 2 {
		t.Fatalf("expected two model usage rows, got %+v", got.ModelUsage)
	}
	if got.ModelUsage[0].Model != "ark-code-latest" || got.ModelUsage[0].Share != float64(200)/float64(300) {
		t.Fatalf("unexpected first model usage row: %+v", got.ModelUsage[0])
	}
	if got.ModelUsage[1].Share != float64(100)/float64(300) {
		t.Fatalf("unexpected second model usage row: %+v", got.ModelUsage[1])
	}

	queries := fakeAdminSQLSnapshot()
	if len(queries) != 3 {
		t.Fatalf("expected 3 queries, got %d", len(queries))
	}
	monthStart, monthEnd := monthWindow(now)
	assertTimeArg(t, queries[0].args[0], monthStart)
	assertTimeArg(t, queries[0].args[1], monthEnd)
	assertTimeArg(t, queries[1].args[0], now.AddDate(0, 0, -30))
	assertTimeArg(t, queries[1].args[1], now)
	if !strings.Contains(queries[0].query, "usage_events ue") || !strings.Contains(queries[0].query, "ue.created_at >= $1") {
		t.Fatalf("summary query should filter on usage_events.created_at: %s", queries[0].query)
	}
	if !strings.Contains(queries[1].query, "GROUP BY (ue.created_at AT TIME ZONE 'UTC')::date") {
		t.Fatalf("trend query should group by UTC date: %s", queries[1].query)
	}
	if !strings.Contains(queries[2].query, "COALESCE(NULLIF(ue.model_routed, ''), NULLIF(ue.model_requested, ''), 'unknown')") {
		t.Fatalf("model query should group by model fallback expression: %s", queries[2].query)
	}
}

func TestPostgresOverviewStoreLoadOverviewGuardsZeroTokenShare(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{"total_tokens", "estimated_cost_usd", "active_users"},
			rows: [][]driver.Value{
				{int64(0), float64(0), int64(0)},
			},
		},
		{
			columns: []string{"usage_date", "total_tokens", "cost_usd"},
			rows:    [][]driver.Value{},
		},
		{
			columns: []string{"model", "total_tokens", "cost_usd"},
			rows: [][]driver.Value{
				{"ark-code-latest", int64(0), float64(0)},
			},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadOverview(context.Background(), now)
	if err != nil {
		t.Fatalf("load overview: %v", err)
	}
	if len(got.ModelUsage) != 1 {
		t.Fatalf("expected one model usage row, got %+v", got.ModelUsage)
	}
	if got.ModelUsage[0].Share != 0 {
		t.Fatalf("expected zero share when total tokens is zero, got %+v", got.ModelUsage[0])
	}
}

func TestPostgresOverviewStoreLoadUsersMapsSQLResults(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{"user_id", "email", "display_name", "used_tokens", "used_budget_cents", "monthly_budget_cents", "status"},
			rows: [][]driver.Value{
				{"00000000-0000-0000-0000-000000000201", "admin@democorp.local", "Demo Admin", int64(300), int64(38), int64(100), "active"},
				{"00000000-0000-0000-0000-000000000202", "user01@democorp.local", "Demo User 01", int64(0), int64(0), nil, "disabled"},
			},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadUsers(context.Background(), now)
	if err != nil {
		t.Fatalf("load users: %v", err)
	}

	if len(got.Users) != 2 {
		t.Fatalf("expected two users, got %+v", got.Users)
	}
	if got.Users[0].Email != "admin@democorp.local" || got.Users[0].UsedTokens != 300 ||
		got.Users[0].UsedBudgetCents != 38 || got.Users[0].BudgetCents == nil ||
		*got.Users[0].BudgetCents != 100 || got.Users[0].Quota != 100 {
		t.Fatalf("unexpected first user row: %+v", got.Users[0])
	}
	if got.Users[1].Status != "disabled" || got.Users[1].UsedTokens != 0 || got.Users[1].BudgetCents != nil {
		t.Fatalf("unexpected second user row: %+v", got.Users[1])
	}

	queries := fakeAdminSQLSnapshot()
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
	monthStart, monthEnd := monthWindow(now)
	assertTimeArg(t, queries[0].args[0], monthStart)
	assertTimeArg(t, queries[0].args[1], monthEnd)
	if !strings.Contains(queries[0].query, "FROM users u") || !strings.Contains(queries[0].query, "LEFT JOIN usage_events ue") {
		t.Fatalf("users query should aggregate users through usage events: %s", queries[0].query)
	}
	if !strings.Contains(queries[0].query, "ue.created_at >= $1") || !strings.Contains(queries[0].query, "ue.created_at < $2") {
		t.Fatalf("users query should filter usage rows by month window: %s", queries[0].query)
	}
	if !strings.Contains(queries[0].query, "CEIL(COALESCE(SUM(cl.cost_usd), 0) * 100)") ||
		!strings.Contains(queries[0].query, "Gateway enforcement compares exact cost_usd") {
		t.Fatalf("users query should expose CEIL display semantics and exact enforcement comment: %s", queries[0].query)
	}
}

func TestPostgresOverviewStoreLoadUsersEmptyResults(t *testing.T) {
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{"user_id", "email", "display_name", "used_tokens", "used_budget_cents", "monthly_budget_cents", "status"},
			rows:    [][]driver.Value{},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadUsers(context.Background(), time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("load users: %v", err)
	}
	if got.Users == nil || len(got.Users) != 0 {
		t.Fatalf("expected empty users array, got %+v", got.Users)
	}
}

func TestPostgresOverviewStoreUpdateUserBudgetMapsSQL(t *testing.T) {
	userID := uuid.New()
	updatedAt := time.Date(2026, 5, 19, 11, 30, 0, 0, time.UTC)
	budget := int64(50)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{"before_budget_cents", "after_budget_cents"},
			rows:    [][]driver.Value{{int64(100), int64(50)}},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.UpdateUserBudget(context.Background(), userID, &budget, updatedAt)
	if err != nil {
		t.Fatalf("update user budget: %v", err)
	}
	if got.BeforeBudgetCents == nil || *got.BeforeBudgetCents != 100 ||
		got.AfterBudgetCents == nil || *got.AfterBudgetCents != 50 {
		t.Fatalf("unexpected update result: %+v", got)
	}

	queries := fakeAdminSQLSnapshot()
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
	for _, want := range []string{"WITH target AS", "UPDATE users", "monthly_budget_cents = $2", "updated_at = $3"} {
		if !strings.Contains(queries[0].query, want) {
			t.Fatalf("update query missing %q: %s", want, queries[0].query)
		}
	}
	if len(queries[0].args) != 3 || fmt.Sprint(queries[0].args[0]) != userID.String() || queries[0].args[1] != budget {
		t.Fatalf("unexpected update args: %#v", queries[0].args)
	}
	assertTimeArg(t, queries[0].args[2], updatedAt)
}

func TestPostgresOverviewStoreUpdateUserBudgetNotFound(t *testing.T) {
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{"before_budget_cents", "after_budget_cents"},
			rows:    [][]driver.Value{},
		},
	})
	store := &postgresOverviewStore{db: db}

	err := func() error {
		_, err := store.UpdateUserBudget(context.Background(), uuid.New(), nil, time.Now())
		return err
	}()
	if !errors.Is(err, errAdminUserNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestPostgresOverviewStoreLoadModelsMapsSQLResults(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{
				"model",
				"provider",
				"prompt_tokens",
				"completion_tokens",
				"total_tokens",
				"cost_usd",
				"call_count",
			},
			rows: [][]driver.Value{
				{"glm-5.1", "ark", int64(100), int64(50), int64(150), float64(0.0001), int64(2)},
				{"unknown", "unknown", int64(1), int64(0), int64(1), float64(0), int64(1)},
			},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadModels(context.Background(), now)
	if err != nil {
		t.Fatalf("load models: %v", err)
	}

	if len(got.Models) != 2 {
		t.Fatalf("expected two model rows, got %+v", got.Models)
	}
	if got.Models[0].Model != "glm-5.1" || got.Models[0].Provider != "ark" || got.Models[0].TotalTokens != 150 {
		t.Fatalf("unexpected first model row: %+v", got.Models[0])
	}
	if got.Models[0].PromptTokens != 100 || got.Models[0].CompletionTokens != 50 || got.Models[0].CallCount != 2 {
		t.Fatalf("unexpected first model counters: %+v", got.Models[0])
	}
	if got.Models[1].Provider != "unknown" || got.Models[1].CostUSD != 0 {
		t.Fatalf("unexpected second model row: %+v", got.Models[1])
	}

	queries := fakeAdminSQLSnapshot()
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
	monthStart, monthEnd := monthWindow(now)
	assertTimeArg(t, queries[0].args[0], monthStart)
	assertTimeArg(t, queries[0].args[1], monthEnd)
	if !strings.Contains(queries[0].query, "GROUP BY") || !strings.Contains(queries[0].query, "model_routed") {
		t.Fatalf("models query should group by model fallback expression: %s", queries[0].query)
	}
	if !strings.Contains(queries[0].query, "COUNT(*)::bigint AS call_count") {
		t.Fatalf("models query should expose call_count: %s", queries[0].query)
	}
}

func TestPostgresOverviewStoreLoadModelsEmptyResults(t *testing.T) {
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{
				"model",
				"provider",
				"prompt_tokens",
				"completion_tokens",
				"total_tokens",
				"cost_usd",
				"call_count",
			},
			rows: [][]driver.Value{},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadModels(context.Background(), time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("load models: %v", err)
	}
	if got.Models == nil || len(got.Models) != 0 {
		t.Fatalf("expected empty models array, got %+v", got.Models)
	}
}

func TestPostgresOverviewStoreLoadAuditLogsMapsSQLResults(t *testing.T) {
	createdAt := time.Date(2026, 5, 19, 9, 30, 0, 123, time.UTC)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: auditLogColumns(),
			rows: [][]driver.Value{
				{
					"bootstrap",
					"bootstrap",
					"create_virtual_key",
					"virtual_key",
					"key-1",
					nil,
					`{"key_prefix":"abcdefghijkl"}`,
					"192.0.2.10",
					"admin-test",
					"req-1",
					int64(201),
					createdAt,
				},
			},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadAuditLogs(context.Background(), auditLogFilters{Limit: 10})
	if err != nil {
		t.Fatalf("load audit logs: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected one audit log, got %+v", got)
	}
	row := got[0]
	if row.ActorID != "bootstrap" || row.ActorType != "bootstrap" || row.Action != "create_virtual_key" {
		t.Fatalf("unexpected actor/action: %+v", row)
	}
	if row.ResourceID == nil || *row.ResourceID != "key-1" || row.IP == nil || *row.IP != "192.0.2.10" {
		t.Fatalf("unexpected nullable fields: %+v", row)
	}
	if row.Before != nil || string(row.After) != `{"key_prefix":"abcdefghijkl"}` {
		t.Fatalf("unexpected snapshots: before=%s after=%s", row.Before, row.After)
	}
	if row.StatusCode != http.StatusCreated || row.CreatedAt != createdAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("unexpected status/created_at: %+v", row)
	}
}

func TestPostgresOverviewStoreLoadAuditLogsBuildsFilteredQuery(t *testing.T) {
	since := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	until := since.Add(time.Hour)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: auditLogColumns(),
			rows:    [][]driver.Value{},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadAuditLogs(context.Background(), auditLogFilters{
		ActorID:      "bootstrap",
		ResourceType: "virtual_key",
		ResourceID:   "key-1",
		Since:        &since,
		Until:        &until,
		Limit:        50,
	})
	if err != nil {
		t.Fatalf("load audit logs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no audit logs, got %+v", got)
	}

	queries := fakeAdminSQLSnapshot()
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
	query := queries[0].query
	for _, want := range []string{
		"FROM audit_logs",
		"actor_id = $1",
		"resource_type = $2",
		"resource_id = $3",
		"created_at >= $4",
		"created_at < $5",
		"LIMIT $6",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("audit logs query missing %q: %s", want, query)
		}
	}
	args := queries[0].args
	if len(args) != 6 || args[0] != "bootstrap" || args[1] != "virtual_key" || args[2] != "key-1" || args[5] != int64(50) && args[5] != 50 {
		t.Fatalf("unexpected audit log query args: %#v", args)
	}
	assertTimeArg(t, args[3], since)
	assertTimeArg(t, args[4], until)
}

func TestOverviewCORSPreflight(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodOptions, "/api/admin/overview", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatal("expected CORS header")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PATCH, DELETE, OPTIONS" {
		t.Fatalf("unexpected CORS methods: %q", got)
	}
}

func TestOverviewCORSDeniesUnlistedOrigin(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	newMux(testLogger(), cfg, nil, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unexpected CORS header: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != "" {
		t.Fatalf("unexpected allowed headers: %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestDevVirtualKeyEndpointRequiresAuthWithoutBootstrapToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, &fakeVirtualKeyCreator{}, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestDevVirtualKeyEndpointRequiresBootstrapToken(t *testing.T) {
	t.Parallel()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, nil, &fakeVirtualKeyCreator{}, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	var body map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"]["code"] != "invalid_api_key" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestDevVirtualKeyEndpointCreatesKey(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	createdAt := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	creator := &fakeVirtualKeyCreator{
		result: createVirtualKeyResult{
			APIKeyID:       uuid.New(),
			OrganizationID: orgID,
			UserID:         userID,
			KeyPrefix:      "abcdefghijkl",
			VirtualKey:     "omt_abcdefghijkl_secret",
			CreatedAt:      createdAt,
		},
	}
	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	body := `{"organization_id":"` + orgID.String() + `","user_id":"` + userID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, nil, creator, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if creator.params.OrganizationID != orgID || creator.params.UserID != userID {
		t.Fatalf("creator params mismatch: %#v", creator.params)
	}
	var response createVirtualKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.DevOnly {
		t.Fatal("expected dev_only=true")
	}
	if response.VirtualKey != creator.result.VirtualKey || response.KeyPrefix != creator.result.KeyPrefix {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestDevVirtualKeyEndpointCreatesAuditRecord(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	apiKeyID := uuid.New()
	creator := &fakeVirtualKeyCreator{
		result: createVirtualKeyResult{
			APIKeyID:       apiKeyID,
			OrganizationID: orgID,
			UserID:         userID,
			KeyPrefix:      "abcdefghijkl",
			VirtualKey:     "omt_abcdefghijkl_secret",
			CreatedAt:      time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
		},
	}
	recorder := newAdminAuditRecorder()
	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	body := `{"organization_id":"` + orgID.String() + `","user_id":"` + userID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	req.Header.Set("User-Agent", "admin-test")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, nil, creator, nil, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	entry := recorder.wait(t)
	if entry.Action != "create_virtual_key" || entry.ResourceType != "virtual_key" || entry.ResourceID != apiKeyID.String() {
		t.Fatalf("unexpected audit resource: %+v", entry)
	}
	if entry.Actor.ID != "bootstrap" || entry.Actor.Type != audit.ActorTypeBootstrap {
		t.Fatalf("unexpected audit actor: %+v", entry.Actor)
	}
	if entry.StatusCode != http.StatusCreated || entry.UserAgent != "admin-test" {
		t.Fatalf("unexpected audit status/user agent: %+v", entry)
	}
	if entry.Before != nil {
		t.Fatalf("expected nil before snapshot, got %#v", entry.Before)
	}
	after, ok := entry.After.(map[string]string)
	if !ok {
		t.Fatalf("unexpected after snapshot type: %T", entry.After)
	}
	if after["key_prefix"] != "abcdefghijkl" || after["api_key_id"] != apiKeyID.String() {
		t.Fatalf("unexpected after snapshot: %#v", after)
	}
	if _, leaked := after["virtual_key"]; leaked || strings.Contains(strings.Join(mapValues(after), " "), "secret") {
		t.Fatalf("audit after snapshot leaked secret: %#v", after)
	}
}

func TestDevVirtualKeyUnauthorizedSkipsAudit(t *testing.T) {
	t.Parallel()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	recorder := newAdminAuditRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, nil, &fakeVirtualKeyCreator{}, nil, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	recorder.expectNoRecord(t)
}

func TestDevVirtualKeyInvalidBodyCreatesFailedAuditAttempt(t *testing.T) {
	t.Parallel()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	recorder := newAdminAuditRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(`{`))
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, nil, &fakeVirtualKeyCreator{}, nil, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	entry := recorder.wait(t)
	if entry.Action != "" || entry.ResourceType != "" {
		t.Fatalf("handler should not set action/resource before invalid body: %+v", entry)
	}
	if entry.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected failed audit status %d, got %d", http.StatusBadRequest, entry.StatusCode)
	}
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()

	var body map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"]["code"] != want {
		t.Fatalf("expected error code %q, got %#v", want, body)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestKeyAnomalyThresholdEnvOverride(t *testing.T) {
	t.Setenv(keyAnomalyThresholdEnv, "42")

	if got := keyAnomalyThreshold(testLogger()); got != 42 {
		t.Fatalf("expected env threshold 42, got %d", got)
	}
}

func TestKeyAnomalyThresholdDefault(t *testing.T) {
	t.Setenv(keyAnomalyThresholdEnv, "")

	if got := keyAnomalyThreshold(testLogger()); got != usageanomaly.DefaultThreshold {
		t.Fatalf("expected default threshold %d, got %d", usageanomaly.DefaultThreshold, got)
	}
}

func TestKeyAnomalyThresholdInvalidEnvFallsBack(t *testing.T) {
	t.Setenv(keyAnomalyThresholdEnv, "0")

	if got := keyAnomalyThreshold(testLogger()); got != usageanomaly.DefaultThreshold {
		t.Fatalf("expected default threshold %d, got %d", usageanomaly.DefaultThreshold, got)
	}
}

func TestKeyAnomalyThresholdInvalidStringFallsBack(t *testing.T) {
	t.Setenv(keyAnomalyThresholdEnv, "abc")

	if got := keyAnomalyThreshold(testLogger()); got != usageanomaly.DefaultThreshold {
		t.Fatalf("expected default threshold %d, got %d", usageanomaly.DefaultThreshold, got)
	}
}

func TestStartKeyAnomalyMonitorNoopsWithoutDB(t *testing.T) {
	t.Parallel()

	startKeyAnomalyMonitor(context.Background(), testLogger(), nil)
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func auditLogColumns() []string {
	return []string{
		"actor_id",
		"actor_type",
		"action",
		"resource_type",
		"resource_id",
		"before",
		"after",
		"ip",
		"user_agent",
		"request_id",
		"status_code",
		"created_at",
	}
}

func testAdminConfig() config.AdminConfig {
	return config.AdminConfig{
		Addr:        ":8081",
		CORSOrigins: []string{"http://localhost:3000"},
		CORSMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
	}
}

type fakeVirtualKeyCreator struct {
	params createVirtualKeyParams
	result createVirtualKeyResult
	err    error
}

func (f *fakeVirtualKeyCreator) CreateVirtualKey(_ context.Context, params createVirtualKeyParams) (createVirtualKeyResult, error) {
	f.params = params
	return f.result, f.err
}

type adminAuditRecorder struct {
	entries chan audit.Entry
	err     error
}

func newAdminAuditRecorder() *adminAuditRecorder {
	return &adminAuditRecorder{entries: make(chan audit.Entry, 1)}
}

func (r *adminAuditRecorder) Record(_ context.Context, entry audit.Entry) error {
	if r.err != nil {
		return r.err
	}
	r.entries <- entry
	return nil
}

func (r *adminAuditRecorder) wait(t *testing.T) audit.Entry {
	t.Helper()

	select {
	case entry := <-r.entries:
		return entry
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for audit entry")
		return audit.Entry{}
	}
}

func (r *adminAuditRecorder) expectNoRecord(t *testing.T) {
	t.Helper()

	select {
	case entry := <-r.entries:
		t.Fatalf("unexpected audit entry: %+v", entry)
	case <-time.After(50 * time.Millisecond):
	}
}

type fakeOverviewStore struct {
	response          overviewResponse
	users             usersResponse
	models            modelsResponse
	auditLogs         []adminAuditLog
	updateResult      updateUserBudgetResult
	err               error
	usersErr          error
	modelsErr         error
	auditLogsErr      error
	updateErr         error
	called            bool
	usersCalled       bool
	modelsCalled      bool
	auditLogsCalled   bool
	updateCalled      bool
	now               time.Time
	usersNow          time.Time
	modelsNow         time.Time
	auditLogFilters   auditLogFilters
	updateUserID      uuid.UUID
	updateBudgetCents *int64
	updateAt          time.Time
	authUserID        uuid.UUID
	authOrgID         uuid.UUID
	authRole          string
	authErr           error
	credentials       credentialsResponse
	credentialsErr    error
}

func (f *fakeOverviewStore) Authenticate(ctx context.Context, email, password string) (uuid.UUID, uuid.UUID, string, error) {
	role := f.authRole
	if role == "" {
		role = "admin"
	}
	return f.authUserID, f.authOrgID, role, f.authErr
}

func (f *fakeOverviewStore) LoadVirtualModels(ctx context.Context) (virtualModelsResponse, error) {
	return virtualModelsResponse{}, errors.New("not implemented")
}

func (f *fakeOverviewStore) LoadCredentials(ctx context.Context) (credentialsResponse, error) {
	return f.credentials, f.credentialsErr
}

func (f *fakeOverviewStore) LoadOverview(_ context.Context, now time.Time) (overviewResponse, error) {
	f.called = true
	f.now = now
	return f.response, f.err
}

func (f *fakeOverviewStore) LoadUsers(_ context.Context, now time.Time) (usersResponse, error) {
	f.usersCalled = true
	f.usersNow = now
	return f.users, f.usersErr
}

func (f *fakeOverviewStore) LoadModels(_ context.Context, now time.Time) (modelsResponse, error) {
	f.modelsCalled = true
	f.modelsNow = now
	return f.models, f.modelsErr
}

func (f *fakeOverviewStore) LoadAuditLogs(_ context.Context, filters auditLogFilters) ([]adminAuditLog, error) {
	f.auditLogsCalled = true
	f.auditLogFilters = filters
	return f.auditLogs, f.auditLogsErr
}

func (f *fakeOverviewStore) UpdateUserBudget(_ context.Context, userID uuid.UUID, budgetCents *int64, updatedAt time.Time) (updateUserBudgetResult, error) {
	f.updateCalled = true
	f.updateUserID = userID
	f.updateBudgetCents = budgetCents
	f.updateAt = updatedAt
	return f.updateResult, f.updateErr
}

func int64Ptr(value int64) *int64 {
	return &value
}

func assertTimeArg(t *testing.T, value driver.Value, want time.Time) {
	t.Helper()

	got, ok := value.(time.Time)
	if !ok {
		t.Fatalf("expected time arg %s, got %T %[2]v", want, value)
	}
	if !got.Equal(want) {
		t.Fatalf("expected time arg %s, got %s", want, got)
	}
}

var adminFakeSQL = &adminFakeSQLState{}
var adminFakeSQLRegisterOnce sync.Once

type adminFakeSQLState struct {
	mu        sync.Mutex
	responses []adminFakeSQLResponse
	queries   []adminFakeSQLQuery
}

type adminFakeSQLResponse struct {
	columns []string
	rows    [][]driver.Value
	err     error
}

type adminFakeSQLQuery struct {
	query string
	args  []driver.Value
}

func openFakeAdminDB(t *testing.T, responses []adminFakeSQLResponse) *sql.DB {
	t.Helper()

	adminFakeSQLRegisterOnce.Do(func() {
		sql.Register("admin_fake_postgres", adminFakeDriver{})
	})

	adminFakeSQL.mu.Lock()
	adminFakeSQL.responses = responses
	adminFakeSQL.queries = nil
	adminFakeSQL.mu.Unlock()

	db, err := sql.Open("admin_fake_postgres", "")
	if err != nil {
		t.Fatalf("open fake admin db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close fake admin db: %v", err)
		}
	})
	return db
}

func fakeAdminSQLSnapshot() []adminFakeSQLQuery {
	adminFakeSQL.mu.Lock()
	defer adminFakeSQL.mu.Unlock()

	queries := make([]adminFakeSQLQuery, len(adminFakeSQL.queries))
	for i, query := range adminFakeSQL.queries {
		args := make([]driver.Value, len(query.args))
		copy(args, query.args)
		queries[i] = adminFakeSQLQuery{query: query.query, args: args}
	}
	return queries
}

type adminFakeDriver struct{}

func (adminFakeDriver) Open(_ string) (driver.Conn, error) {
	return adminFakeConn{}, nil
}

type adminFakeConn struct{}

func (adminFakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("admin fake driver does not support prepared statements")
}

func (adminFakeConn) Close() error {
	return nil
}

func (adminFakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("admin fake driver does not support transactions")
}

func (adminFakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	adminFakeSQL.mu.Lock()
	defer adminFakeSQL.mu.Unlock()

	index := len(adminFakeSQL.queries)
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	adminFakeSQL.queries = append(adminFakeSQL.queries, adminFakeSQLQuery{query: query, args: values})

	if index >= len(adminFakeSQL.responses) {
		return nil, errors.New("unexpected query")
	}
	response := adminFakeSQL.responses[index]
	if response.err != nil {
		return nil, response.err
	}
	return &adminFakeRows{
		columns: response.columns,
		rows:    response.rows,
	}, nil
}

type adminFakeRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *adminFakeRows) Columns() []string {
	return r.columns
}

func (r *adminFakeRows) Close() error {
	return nil
}

func (r *adminFakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func TestAdminLoginSuccess(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	orgID := uuid.New()
	store := &fakeOverviewStore{
		authUserID: userID,
		authOrgID:  orgID,
	}
	sessionStore := auth.NewSessionStore(time.Hour)
	recorder := newAdminAuditRecorder()
	cfg := testAdminConfig()

	body := `{"email":"admin@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, store, nil, sessionStore, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp loginResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected token, got empty")
	}

	session, ok := sessionStore.Validate(resp.Token)
	if !ok {
		t.Fatal("session not validated")
	}
	if session.UserID != userID || session.OrgID != orgID {
		t.Fatalf("session mismatch: %+v", session)
	}

	entry := recorder.wait(t)
	if entry.Action != "login" || entry.Actor.Type != "anonymous" {
		t.Fatalf("unexpected audit entry: %+v", entry)
	}
}

func TestAdminLoginWrongPwd(t *testing.T) {
	t.Parallel()

	store := &fakeOverviewStore{
		authErr: ErrInvalidCredentials,
	}
	sessionStore := auth.NewSessionStore(time.Hour)
	recorder := newAdminAuditRecorder()
	cfg := testAdminConfig()

	body := `{"email":"admin@example.com","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, store, nil, sessionStore, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}

	entry := recorder.wait(t)
	if entry.Action != "login_failed" || entry.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected audit entry: %+v", entry)
	}
}

func TestAdminLoginDisabled(t *testing.T) {
	t.Parallel()

	store := &fakeOverviewStore{
		authErr: ErrUserDisabled,
	}
	sessionStore := auth.NewSessionStore(time.Hour)
	recorder := newAdminAuditRecorder()
	cfg := testAdminConfig()

	body := `{"email":"admin@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, store, nil, sessionStore, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}

	entry := recorder.wait(t)
	if entry.Action != "login_failed" || entry.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected audit entry: %+v", entry)
	}
}

func TestAdminSessionExpired(t *testing.T) {
	t.Parallel()

	sessionStore := auth.NewSessionStore(-time.Hour) // immediate expiration
	token, _ := sessionStore.Create(uuid.New(), uuid.New(), "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, nil, sessionStore).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestAdminSessionRevoked(t *testing.T) {
	t.Parallel()

	sessionStore := auth.NewSessionStore(time.Hour)
	token, _ := sessionStore.Create(uuid.New(), uuid.New(), "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, nil, sessionStore).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, nil, sessionStore).ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec2.Code)
	}
}

func TestAdminViewerWriteDeniedAndAudited(t *testing.T) {
	t.Parallel()

	sessionStore := auth.NewSessionStore(time.Hour)
	token, _ := sessionStore.Create(uuid.New(), uuid.New(), "viewer")
	recorder := newAdminAuditRecorder()
	cfg := testAdminConfig()
	userID := uuid.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userID.String()+"/quota", strings.NewReader(`{"budget_cents":50}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, &fakeOverviewStore{}, nil, sessionStore, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	entry := recorder.wait(t)
	if entry.Action != rbac.ActionUpdateQuota || entry.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected audit entry: %+v", entry)
	}
}

func TestAdminLoginAuditLogs(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	store := &fakeOverviewStore{
		authUserID: userID,
		authOrgID:  uuid.New(),
	}
	sessionStore := auth.NewSessionStore(time.Hour)
	recorder := newAdminAuditRecorder()
	cfg := testAdminConfig()

	body := `{"email":"admin@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, store, nil, sessionStore, recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	entry := recorder.wait(t)
	after, ok := entry.After.(map[string]string)
	if !ok || after["user_id"] != userID.String() {
		t.Fatalf("unexpected after snapshot: %#v", entry.After)
	}
}
