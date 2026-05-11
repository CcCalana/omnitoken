package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger()).ServeHTTP(rec, req)

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

	newMux(testLogger()).ServeHTTP(rec, req)

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
	rec := httptest.NewRecorder()

	newMux(testLogger()).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("expected CORS header")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
