package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseConfigRejectsRequestsAboveMax(t *testing.T) {
	t.Parallel()

	_, err := parseConfig([]string{
		"-concurrency", "2",
		"-requests", "3",
		"-key", "omt_test",
		"-admin-token", "dev-bootstrap",
	}, func(key string) string {
		if key == "MAX_REQUESTS" {
			return "5"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds MAX_REQUESTS=5") {
		t.Fatalf("expected max requests error, got %v", err)
	}
}

func TestParseConfigUsesAdminTokenFromEnv(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]string{
		"-concurrency", "1",
		"-requests", "1",
		"-key", "omt_test",
	}, func(key string) string {
		if key == "OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN" {
			return "dev-bootstrap"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.adminToken != "dev-bootstrap" {
		t.Fatalf("unexpected admin token: %q", cfg.adminToken)
	}
}

func TestRunSendsRequestsAndChecksOverview(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int64
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer omt_test" {
			t.Fatalf("unexpected gateway auth header: %q", got)
		}
		var body chatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode chat request: %v", err)
		}
		if body.Stream || body.Model != defaultModel || body.MaxTokens != 16 {
			t.Fatalf("unexpected chat request body: %+v", body)
		}
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test"}`))
	}))
	defer gateway.Close()

	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/admin/overview" {
			t.Fatalf("unexpected admin request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dev-bootstrap" {
			t.Fatalf("unexpected admin auth header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_tokens":123,"active_users":1}`))
	}))
	defer admin.Close()

	cfg := config{
		concurrency: 2,
		requests:    3,
		gatewayURL:  gateway.URL,
		adminURL:    admin.URL,
		key:         "omt_test",
		adminToken:  "dev-bootstrap",
		model:       defaultModel,
		timeout:     2 * time.Second,
		maxRequests: defaultMaxRequests,
	}
	var out bytes.Buffer

	if err := run(context.Background(), cfg, gateway.Client(), &out); err != nil {
		t.Fatalf("run loadtest: %v", err)
	}
	if got := requestCount.Load(); got != 6 {
		t.Fatalf("expected 6 gateway requests, got %d", got)
	}
	output := out.String()
	for _, want := range []string{
		"total_requests\t6",
		"success_rate\t100.0%",
		"2xx\t6",
		"usage_total_tokens\t123",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("summary missing %q:\n%s", want, output)
		}
	}
}

func TestRunFailsWhenUsageAssertionFails(t *testing.T) {
	t.Parallel()

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer gateway.Close()

	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_tokens":0,"active_users":0}`))
	}))
	defer admin.Close()

	cfg := config{
		concurrency: 1,
		requests:    1,
		gatewayURL:  gateway.URL,
		adminURL:    admin.URL,
		key:         "omt_test",
		adminToken:  "dev-bootstrap",
		model:       defaultModel,
		timeout:     2 * time.Second,
		maxRequests: defaultMaxRequests,
	}

	err := run(context.Background(), cfg, gateway.Client(), ioDiscard{})
	if err == nil || !strings.Contains(err.Error(), "usage assertion failed") {
		t.Fatalf("expected usage assertion error, got %v", err)
	}
}

func TestRunFailsOnNon2xxRequests(t *testing.T) {
	t.Parallel()

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer gateway.Close()

	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_tokens":1,"active_users":1}`))
	}))
	defer admin.Close()

	cfg := config{
		concurrency: 1,
		requests:    1,
		gatewayURL:  gateway.URL,
		adminURL:    admin.URL,
		key:         "omt_test",
		adminToken:  "dev-bootstrap",
		model:       defaultModel,
		timeout:     2 * time.Second,
		maxRequests: defaultMaxRequests,
	}
	var out bytes.Buffer

	err := run(context.Background(), cfg, gateway.Client(), &out)
	if err == nil || !strings.Contains(err.Error(), "chat requests had failures") {
		t.Fatalf("expected failed request error, got %v", err)
	}
	if !strings.Contains(out.String(), "5xx\t1") {
		t.Fatalf("expected summary to include 5xx count, got:\n%s", out.String())
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
