//go:build e2e

package usage

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/proxy"
)

func TestUsageRecorderE2E(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("OMNITOKEN_ARK_API_KEY"))
	databaseURL := strings.TrimSpace(os.Getenv("OMNITOKEN_TEST_DATABASE_URL"))
	if apiKey == "" || databaseURL == "" {
		t.Skip("OMNITOKEN_ARK_API_KEY and OMNITOKEN_TEST_DATABASE_URL are required for usage e2e")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	subject := insertUsageE2EIdentity(t, db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Middleware(
		NewRecorder(NewPostgresStore(db), logger),
		MiddlewareConfig{
			Provider:      "ark",
			ModelFallback: config.DefaultArkModel,
			Logger:        logger,
		},
	)(proxy.NewArkChatProxy(proxy.ArkChatConfig{
		BaseURL:         config.Env("OMNITOKEN_ARK_OPENAI_BASE_URL", config.DefaultArkOpenAIBaseURL),
		APIKey:          apiKey,
		DefaultModel:    config.Env("OMNITOKEN_ARK_DEFAULT_MODEL", config.DefaultArkModel),
		DisableThinking: true,
	}, logger, nil))

	body := `{"model":"client-model","messages":[{"role":"user","content":"Output exactly: pong"}],"max_tokens":32}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req = req.WithContext(auth.WithSubject(req.Context(), subject))
	rec := httptest.NewRecorder()

	httpx.RequestID(handler).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	requestID := rec.Header().Get(httpx.RequestIDHeader)
	if requestID == "" {
		t.Fatal("missing request id")
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		var usageEvents, costRows int
		err := db.QueryRow(`
SELECT
  COUNT(*) FILTER (WHERE usage_events.request_id = $1),
  COUNT(cost_ledger.id) FILTER (WHERE usage_events.request_id = $1)
FROM usage_events
LEFT JOIN cost_ledger ON cost_ledger.usage_event_id = usage_events.id
`, requestID).Scan(&usageEvents, &costRows)
		if err != nil {
			t.Fatalf("query usage rows: %v", err)
		}
		if usageEvents == 1 && costRows == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for usage ledger rows: usage_events=%d cost_ledger=%d request_id=%s", usageEvents, costRows, requestID)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func insertUsageE2EIdentity(t *testing.T, db *sql.DB) auth.Subject {
	t.Helper()

	orgID := uuid.New()
	userID := uuid.New()
	apiKeyID := uuid.New()
	prefix := "e2e" + strings.ReplaceAll(apiKeyID.String(), "-", "")[:8]

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO organizations (id, name) VALUES ($1, $2)`, orgID, "Usage E2E "+apiKeyID.String()); err != nil {
		t.Fatalf("insert organization: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO users (id, organization_id, email, display_name)
VALUES ($1, $2, $3, 'Usage E2E User')
`, userID, orgID, "usage-e2e-"+apiKeyID.String()+"@example.local"); err != nil {
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
