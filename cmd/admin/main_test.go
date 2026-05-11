package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/config"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil).ServeHTTP(rec, req)

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

func TestOverview(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body overviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode overview response: %v", err)
	}
	if body.Period == "" {
		t.Fatal("expected period")
	}
	if body.TotalTokens == 0 {
		t.Fatal("expected total tokens")
	}
	if len(body.ModelUsage) == 0 {
		t.Fatal("expected model usage")
	}
}

func TestOverviewCORSPreflight(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodOptions, "/api/admin/overview", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil).ServeHTTP(rec, req)

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
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil).ServeHTTP(rec, req)

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

func TestDevVirtualKeyEndpointDisabledWithoutBootstrapToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), &fakeVirtualKeyCreator{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestDevVirtualKeyEndpointRequiresBootstrapToken(t *testing.T) {
	t.Parallel()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, &fakeVirtualKeyCreator{}).ServeHTTP(rec, req)

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

	newMux(testLogger(), cfg, creator).ServeHTTP(rec, req)

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

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
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
