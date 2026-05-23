//go:build e2e

package main

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/credentials"
	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/proxy"
	"github.com/omnitoken/omnitoken/internal/usage"
)

func TestCredentialPoolE2ESwitchesAndAttributesUsage(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("OMNITOKEN_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("OMNITOKEN_TEST_DATABASE_URL is required for credential pool e2e")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	upstream := newCredentialPoolMockUpstream()
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)

	ids := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-0000000000a1"),
		uuid.MustParse("00000000-0000-0000-0000-0000000000b1"),
		uuid.MustParse("00000000-0000-0000-0000-0000000000c1"),
	}
	t.Cleanup(func() {
		deleteCredentialPoolE2ECredentials(t, db, ids...)
		_ = db.Close()
	})
	subject := insertCredentialPoolE2EIdentity(t, db)
	upsertCredentialPoolE2ECredentials(t, db, ids, server.URL)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	selector := credentials.NewSelector([]credentials.Credential{
		{ID: ids[0].String(), Provider: "ark", BaseURL: server.URL, Secret: "seed-a", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		{ID: ids[1].String(), Provider: "ark", BaseURL: server.URL, Secret: "seed-b", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		{ID: ids[2].String(), Provider: "ark", BaseURL: server.URL, Secret: "seed-c", Priority: 10, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
	})
	handler := usage.Middleware(
		usage.NewRecorder(usage.NewPostgresStore(db), logger),
		usage.MiddlewareConfig{
			Provider:      "ark",
			ModelFallback: config.DefaultArkModel,
			Logger:        logger,
		},
	)(proxy.NewArkChatProxy(proxy.ArkChatConfig{
		DefaultModel:         config.DefaultArkModel,
		CredentialSelector:   selector,
		MaxCredentialRetries: 2,
		DegradeDuration:      20 * time.Millisecond,
	}, logger, nil))

	for i := 0; i < 5; i++ {
		if i == 1 {
			time.Sleep(25 * time.Millisecond)
		}
		rec := httptest.NewRecorder()
		httpx.RequestID(handler).ServeHTTP(rec, credentialPoolE2ERequest(subject))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d body=%s", i+1, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "quota_owner") {
			t.Fatalf("request %d leaked upstream 429 body: %s", i+1, rec.Body.String())
		}
	}

	hits := upstream.snapshotHits()
	if hits["seed-a"] < 2 || hits["seed-b"] == 0 || hits["seed-c"] == 0 {
		t.Fatalf("expected all credentials to be used after one 429 switch, hits=%v", hits)
	}
	waitForCredentialUsageCounts(t, db, subject.APIKeyID, ids)
}

func TestCredentialPoolE2ECrossProviderFallback(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("OMNITOKEN_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("OMNITOKEN_TEST_DATABASE_URL is required for credential pool e2e")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	upstream := newCrossProviderMockUpstream()
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)

	arkID := uuid.MustParse("00000000-0000-0000-0000-0000000000d1")
	deepSeekID := uuid.MustParse("00000000-0000-0000-0000-0000000000e1")
	t.Cleanup(func() {
		deleteCredentialPoolE2ECredentials(t, db, arkID, deepSeekID)
		_ = db.Close()
	})
	subject := insertCredentialPoolE2EIdentity(t, db)
	upsertCredentialPoolE2ECredential(t, db, arkID, "ark", server.URL, "cross-provider-ark", 1)
	upsertCredentialPoolE2ECredential(t, db, deepSeekID, "deepseek", server.URL, "cross-provider-deepseek", 2)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	selector := credentials.NewSelector([]credentials.Credential{
		{ID: arkID.String(), Provider: "ark", BaseURL: server.URL, Secret: "ark-429", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		{ID: deepSeekID.String(), Provider: "deepseek", BaseURL: server.URL, Secret: "deepseek-ok", Priority: 2, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
	})
	handler := usage.Middleware(
		usage.NewRecorder(usage.NewPostgresStore(db), logger),
		usage.MiddlewareConfig{
			Provider:      "ark",
			ModelFallback: config.DefaultArkModel,
			Logger:        logger,
		},
	)(proxy.NewArkChatProxy(proxy.ArkChatConfig{
		DefaultModel:         config.DefaultArkModel,
		CredentialSelector:   selector,
		MaxCredentialRetries: 2,
		DegradeDuration:      20 * time.Millisecond,
	}, logger, nil))

	rec := httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, credentialPoolE2ERequestWithProvider(subject, "ark"))
	if rec.Code != http.StatusOK {
		t.Fatalf("ark preferred fallback status = %d body=%s", rec.Code, rec.Body.String())
	}
	waitForCredentialUsageProvider(t, db, subject.APIKeyID, deepSeekID, "deepseek")

	subject = insertCredentialPoolE2EIdentity(t, db)
	selector = credentials.NewSelector([]credentials.Credential{
		{ID: arkID.String(), Provider: "ark", BaseURL: server.URL, Secret: "ark-ok", Priority: 1, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
		{ID: deepSeekID.String(), Provider: "deepseek", BaseURL: server.URL, Secret: "deepseek-429", Priority: 2, Weight: 1, Status: credentials.StatusActive, HealthState: credentials.HealthHealthy},
	})
	handler = usage.Middleware(
		usage.NewRecorder(usage.NewPostgresStore(db), logger),
		usage.MiddlewareConfig{
			Provider:      "ark",
			ModelFallback: config.DefaultArkModel,
			Logger:        logger,
		},
	)(proxy.NewArkChatProxy(proxy.ArkChatConfig{
		DefaultModel:         config.DefaultArkModel,
		CredentialSelector:   selector,
		MaxCredentialRetries: 2,
		DegradeDuration:      20 * time.Millisecond,
	}, logger, nil))

	rec = httptest.NewRecorder()
	httpx.RequestID(handler).ServeHTTP(rec, credentialPoolE2ERequestWithProvider(subject, "deepseek"))
	if rec.Code != http.StatusOK {
		t.Fatalf("deepseek preferred fallback status = %d body=%s", rec.Code, rec.Body.String())
	}
	waitForCredentialUsageProvider(t, db, subject.APIKeyID, arkID, "ark")
}

type credentialPoolMockUpstream struct {
	mu   sync.Mutex
	hits map[string]int
}

func newCredentialPoolMockUpstream() *credentialPoolMockUpstream {
	return &credentialPoolMockUpstream{hits: map[string]int{}}
}

func (u *credentialPoolMockUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	u.mu.Lock()
	u.hits[token]++
	hit := u.hits[token]
	u.mu.Unlock()

	if token == "seed-a" && hit == 1 {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"quota_owner":"must-not-leak"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"model":"ark-code-latest","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
}

func (u *credentialPoolMockUpstream) snapshotHits() map[string]int {
	u.mu.Lock()
	defer u.mu.Unlock()
	out := make(map[string]int, len(u.hits))
	for key, value := range u.hits {
		out[key] = value
	}
	return out
}

type crossProviderMockUpstream struct{}

func newCrossProviderMockUpstream() *crossProviderMockUpstream {
	return &crossProviderMockUpstream{}
}

func (u *crossProviderMockUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if strings.HasSuffix(token, "-429") {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"quota_owner":"must-not-leak"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"model":"deepseek-v4-flash","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
}

func credentialPoolE2ERequest(subject auth.Subject) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"client-model","messages":[]}`))
	return req.WithContext(auth.WithSubject(req.Context(), subject))
}

func credentialPoolE2ERequestWithProvider(subject auth.Subject, provider string) *http.Request {
	req := credentialPoolE2ERequest(subject)
	ctx := httpx.WithProviderRouted(req.Context(), provider)
	return req.WithContext(ctx)
}

func insertCredentialPoolE2EIdentity(t *testing.T, db *sql.DB) auth.Subject {
	t.Helper()

	orgID := uuid.New()
	userID := uuid.New()
	apiKeyID := uuid.New()
	prefix := "pool" + strings.ReplaceAll(apiKeyID.String(), "-", "")[:8]

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO organizations (id, name) VALUES ($1, $2)`, orgID, "Credential Pool E2E "+apiKeyID.String()); err != nil {
		t.Fatalf("insert organization: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO users (id, organization_id, email, display_name)
VALUES ($1, $2, $3, 'Credential Pool E2E User')
`, userID, orgID, "credential-pool-e2e-"+apiKeyID.String()+"@example.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO api_keys (id, organization_id, user_id, key_prefix, key_hash, status)
VALUES ($1, $2, $3, $4, decode('00', 'hex'), 'active')
`, apiKeyID, orgID, userID, prefix); err != nil {
		t.Fatalf("insert api key: %v", err)
	}

	return auth.Subject{
		UserID:   userID,
		OrgID:    orgID,
		APIKeyID: apiKeyID,
	}
}

func upsertCredentialPoolE2ECredentials(t *testing.T, db *sql.DB, ids []uuid.UUID, baseURL string) {
	t.Helper()

	for i, id := range ids {
		alias := "credential-pool-e2e-" + string(rune('a'+i))
		upsertCredentialPoolE2ECredential(t, db, id, "ark", baseURL, alias, 10)
	}
}

func upsertCredentialPoolE2ECredential(t *testing.T, db *sql.DB, id uuid.UUID, provider string, baseURL string, alias string, priority int) {
	t.Helper()

	if _, err := db.ExecContext(context.Background(), `
INSERT INTO upstream_credentials (
  id,
  provider,
  base_url,
  encrypted_secret,
  priority,
  weight,
  status,
  health_state,
  metadata
)
VALUES ($1, $2, $3, decode('00', 'hex'), $4, 1, 'active', 'healthy', jsonb_build_object('alias', $5::text))
ON CONFLICT (id) DO UPDATE
SET base_url = EXCLUDED.base_url,
    provider = EXCLUDED.provider,
    priority = EXCLUDED.priority,
    weight = EXCLUDED.weight,
    status = EXCLUDED.status,
    health_state = EXCLUDED.health_state,
    metadata = EXCLUDED.metadata,
    updated_at = now()
`, id, provider, baseURL, priority, alias); err != nil {
		t.Fatalf("upsert credential %s: %v", id, err)
	}
}

func waitForCredentialUsageCounts(t *testing.T, db *sql.DB, apiKeyID uuid.UUID, ids []uuid.UUID) {
	t.Helper()

	want := map[string]struct{}{}
	for _, id := range ids {
		want[id.String()] = struct{}{}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		rows, err := db.QueryContext(context.Background(), `
SELECT upstream_credential_id::text, COUNT(*)
FROM usage_events
WHERE api_key_id = $1
GROUP BY upstream_credential_id
`, apiKeyID)
		if err != nil {
			t.Fatalf("query usage credential counts: %v", err)
		}
		got := map[string]int{}
		for rows.Next() {
			var id string
			var count int
			if err := rows.Scan(&id, &count); err != nil {
				_ = rows.Close()
				t.Fatalf("scan usage credential count: %v", err)
			}
			got[id] = count
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close usage credential rows: %v", err)
		}
		if len(got) == len(want) {
			allPresent := true
			for id := range want {
				if got[id] == 0 {
					allPresent = false
					break
				}
			}
			if allPresent {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for credential usage counts: got=%v want_ids=%v", got, ids)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitForCredentialUsageProvider(t *testing.T, db *sql.DB, apiKeyID uuid.UUID, credentialID uuid.UUID, provider string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		var count int
		err := db.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM usage_events
WHERE api_key_id = $1
  AND upstream_credential_id = $2
  AND provider = $3
`, apiKeyID, credentialID, provider).Scan(&count)
		if err != nil {
			t.Fatalf("query usage provider: %v", err)
		}
		if count > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for provider %s credential %s usage", provider, credentialID)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func deleteCredentialPoolE2ECredentials(t *testing.T, db *sql.DB, ids ...uuid.UUID) {
	t.Helper()

	for _, id := range ids {
		if _, err := db.ExecContext(context.Background(), `DELETE FROM upstream_credentials WHERE id = $1`, id); err != nil {
			t.Fatalf("delete e2e credential %s: %v", id, err)
		}
	}
}
