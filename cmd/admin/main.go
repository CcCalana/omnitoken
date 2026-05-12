package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/httpx"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Time    string `json:"time"`
}

type overviewResponse struct {
	Period            string            `json:"period"`
	TotalTokens       int64             `json:"total_tokens"`
	EstimatedCostUSD  float64           `json:"estimated_cost_usd"`
	ActiveUsers       int               `json:"active_users"`
	QuotaWarningUsers int               `json:"quota_warning_users"`
	Trend             []dailyTokenUsage `json:"trend"`
	ModelUsage        []modelUsage      `json:"model_usage"`
}

type dailyTokenUsage struct {
	Date   string  `json:"date"`
	Tokens int64   `json:"tokens"`
	Cost   float64 `json:"cost_usd"`
}

type modelUsage struct {
	Model  string  `json:"model"`
	Tokens int64   `json:"tokens"`
	Cost   float64 `json:"cost_usd"`
	Share  float64 `json:"share"`
}

type usersResponse struct {
	Users []adminUserUsage `json:"users"`
}

type adminUserUsage struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	UsedTokens  int64  `json:"used_tokens"`
	Quota       int64  `json:"quota"`
	Status      string `json:"status"`
}

type modelsResponse struct {
	Models []adminModelUsage `json:"models"`
}

type adminModelUsage struct {
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	CallCount        int64   `json:"call_count"`
}

type createVirtualKeyRequest struct {
	OrganizationID string     `json:"organization_id"`
	UserID         string     `json:"user_id"`
	ProjectID      string     `json:"project_id,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type createVirtualKeyResponse struct {
	DevOnly        bool   `json:"dev_only"`
	APIKeyID       string `json:"api_key_id"`
	OrganizationID string `json:"organization_id"`
	UserID         string `json:"user_id"`
	KeyPrefix      string `json:"key_prefix"`
	VirtualKey     string `json:"virtual_key"`
	CreatedAt      string `json:"created_at"`
}

type createVirtualKeyParams struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	ProjectID      uuid.NullUUID
	ExpiresAt      *time.Time
}

type createVirtualKeyResult struct {
	APIKeyID       uuid.UUID
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	KeyPrefix      string
	VirtualKey     string
	CreatedAt      time.Time
}

type virtualKeyCreator interface {
	CreateVirtualKey(context.Context, createVirtualKeyParams) (createVirtualKeyResult, error)
}

type overviewStore interface {
	LoadOverview(context.Context, time.Time) (overviewResponse, error)
	LoadUsers(context.Context, time.Time) (usersResponse, error)
	LoadModels(context.Context, time.Time) (modelsResponse, error)
}

type postgresVirtualKeyCreator struct {
	db     *sql.DB
	random io.Reader
	now    func() time.Time
}

type postgresOverviewStore struct {
	db *sql.DB
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db, closeDB := newAdminDB(logger, cfg)
	defer closeDB()
	creator := newVirtualKeyCreator(logger, cfg, db)
	overview := newOverviewStore(db)

	server := &http.Server{
		Addr:              cfg.Admin.Addr,
		Handler:           newMux(logger, cfg.Admin, overview, creator),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("admin listening", "addr", cfg.Admin.Addr)
	if err := httpx.Run(context.Background(), server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("admin stopped", "error", err)
		os.Exit(1)
	}
}

func newAdminDB(logger *slog.Logger, cfg config.Config) (*sql.DB, func()) {
	if cfg.DatabaseURL == "" {
		logger.Info("admin database disabled", "reason", "database_url_empty")
		return nil, func() {}
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	return db, func() {
		if err := db.Close(); err != nil {
			logger.Error("close postgres", "error", err)
		}
	}
}

func newOverviewStore(db *sql.DB) overviewStore {
	if db == nil {
		return nil
	}
	return &postgresOverviewStore{db: db}
}

func newVirtualKeyCreator(logger *slog.Logger, cfg config.Config, db *sql.DB) virtualKeyCreator {
	if cfg.Admin.BootstrapToken == "" {
		logger.Info("admin dev virtual key endpoint disabled", "reason", "bootstrap_token_empty", "port", cfg.Admin.Addr)
		return nil
	}
	if db == nil {
		logger.Error("admin dev virtual key endpoint requires OMNITOKEN_DATABASE_URL")
		os.Exit(1)
	}

	return &postgresVirtualKeyCreator{db: db, random: nil, now: time.Now}
}

func newMux(logger *slog.Logger, cfg config.AdminConfig, overview overviewStore, creator virtualKeyCreator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /api/admin/overview", makeOverviewHandler(logger, overview, time.Now))
	mux.HandleFunc("GET /api/admin/users", makeUsersHandler(logger, overview, time.Now))
	mux.HandleFunc("GET /api/admin/models", makeModelsHandler(logger, overview, time.Now))
	if cfg.BootstrapToken != "" && creator != nil {
		// Demo-Ready 简化版: this admin-port-only endpoint lives on
		// 8081, not the gateway data-plane port 8080. Full admin auth/RBAC and
		// audit_logs writes stay in T-005b.
		mux.HandleFunc("POST /api/admin/dev/virtual-keys", makeCreateVirtualKeyHandler(logger, cfg.BootstrapToken, creator))
	}

	return httpx.RequestID(httpx.RequestLogger(logger)(httpx.CORS(cfg.CORSOrigins, cfg.CORSMethods)(mux)))
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "admin",
		Time:    time.Now().UTC().Format(time.RFC3339),
	})
}

func makeOverviewHandler(logger *slog.Logger, store overviewStore, now func() time.Time) http.HandlerFunc {
	if now == nil {
		now = time.Now
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			httpx.WriteJSON(w, http.StatusOK, zeroOverview(now()))
			return
		}

		overview, err := store.LoadOverview(r.Context(), now())
		if err != nil {
			logger.Error("load overview", "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to load overview", "server_error", "overview_query_failed"))
			return
		}

		httpx.WriteJSON(w, http.StatusOK, overview)
	}
}

func makeUsersHandler(logger *slog.Logger, store overviewStore, now func() time.Time) http.HandlerFunc {
	if now == nil {
		now = time.Now
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			httpx.WriteJSON(w, http.StatusOK, emptyUsersResponse())
			return
		}

		users, err := store.LoadUsers(r.Context(), now())
		if err != nil {
			logger.Error("load admin users", "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to load users", "server_error", "users_query_failed"))
			return
		}

		httpx.WriteJSON(w, http.StatusOK, users)
	}
}

func makeModelsHandler(logger *slog.Logger, store overviewStore, now func() time.Time) http.HandlerFunc {
	if now == nil {
		now = time.Now
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			httpx.WriteJSON(w, http.StatusOK, emptyModelsResponse())
			return
		}

		models, err := store.LoadModels(r.Context(), now())
		if err != nil {
			logger.Error("load admin models", "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to load models", "server_error", "models_query_failed"))
			return
		}

		httpx.WriteJSON(w, http.StatusOK, models)
	}
}

func zeroOverview(now time.Time) overviewResponse {
	return overviewResponse{
		Period:            now.UTC().Format("2006-01"),
		QuotaWarningUsers: 0,
		Trend:             []dailyTokenUsage{},
		ModelUsage:        []modelUsage{},
	}
}

func emptyUsersResponse() usersResponse {
	return usersResponse{Users: []adminUserUsage{}}
}

func emptyModelsResponse() modelsResponse {
	return modelsResponse{Models: []adminModelUsage{}}
}

func (s *postgresOverviewStore) LoadOverview(ctx context.Context, now time.Time) (overviewResponse, error) {
	now = now.UTC()
	monthStart, monthEnd := monthWindow(now)
	response := zeroOverview(now)

	const summaryQuery = `
SELECT
  COALESCE(SUM(utb.total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(cl.cost_usd), 0)::float8 AS estimated_cost_usd,
  COUNT(DISTINCT ue.user_id) FILTER (
    WHERE ue.status_code BETWEEN 200 AND 299 AND ue.user_id IS NOT NULL
  )::bigint AS active_users
FROM usage_events ue
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
LEFT JOIN cost_ledger cl ON cl.usage_event_id = ue.id
WHERE ue.created_at >= $1 AND ue.created_at < $2`

	var activeUsers int64
	if err := s.db.QueryRowContext(ctx, summaryQuery, monthStart, monthEnd).Scan(
		&response.TotalTokens,
		&response.EstimatedCostUSD,
		&activeUsers,
	); err != nil {
		return overviewResponse{}, fmt.Errorf("query overview summary: %w", err)
	}
	response.ActiveUsers = int(activeUsers)

	trend, err := s.loadTrend(ctx, now.AddDate(0, 0, -30), now)
	if err != nil {
		return overviewResponse{}, err
	}
	response.Trend = trend

	modelUsage, err := s.loadModelUsage(ctx, monthStart, monthEnd, response.TotalTokens)
	if err != nil {
		return overviewResponse{}, err
	}
	response.ModelUsage = modelUsage

	return response, nil
}

func monthWindow(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

func (s *postgresOverviewStore) loadTrend(ctx context.Context, start time.Time, end time.Time) ([]dailyTokenUsage, error) {
	const trendQuery = `
SELECT
  to_char((ue.created_at AT TIME ZONE 'UTC')::date, 'YYYY-MM-DD') AS usage_date,
  COALESCE(SUM(utb.total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(cl.cost_usd), 0)::float8 AS cost_usd
FROM usage_events ue
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
LEFT JOIN cost_ledger cl ON cl.usage_event_id = ue.id
WHERE ue.created_at >= $1 AND ue.created_at < $2
GROUP BY (ue.created_at AT TIME ZONE 'UTC')::date
ORDER BY (ue.created_at AT TIME ZONE 'UTC')::date ASC`

	rows, err := s.db.QueryContext(ctx, trendQuery, start.UTC(), end.UTC())
	if err != nil {
		return nil, fmt.Errorf("query overview trend: %w", err)
	}
	defer rows.Close()

	trend := []dailyTokenUsage{}
	for rows.Next() {
		var item dailyTokenUsage
		if err := rows.Scan(&item.Date, &item.Tokens, &item.Cost); err != nil {
			return nil, fmt.Errorf("scan overview trend: %w", err)
		}
		trend = append(trend, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate overview trend: %w", err)
	}
	return trend, nil
}

func (s *postgresOverviewStore) loadModelUsage(ctx context.Context, start time.Time, end time.Time, totalTokens int64) ([]modelUsage, error) {
	const modelUsageQuery = `
SELECT
  COALESCE(NULLIF(ue.model_actual, ''), ue.model_requested, 'unknown') AS model,
  COALESCE(SUM(utb.total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(cl.cost_usd), 0)::float8 AS cost_usd
FROM usage_events ue
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
LEFT JOIN cost_ledger cl ON cl.usage_event_id = ue.id
WHERE ue.created_at >= $1 AND ue.created_at < $2
GROUP BY COALESCE(NULLIF(ue.model_actual, ''), ue.model_requested, 'unknown')
ORDER BY total_tokens DESC, model ASC`

	rows, err := s.db.QueryContext(ctx, modelUsageQuery, start.UTC(), end.UTC())
	if err != nil {
		return nil, fmt.Errorf("query overview model usage: %w", err)
	}
	defer rows.Close()

	usage := []modelUsage{}
	for rows.Next() {
		var item modelUsage
		if err := rows.Scan(&item.Model, &item.Tokens, &item.Cost); err != nil {
			return nil, fmt.Errorf("scan overview model usage: %w", err)
		}
		if totalTokens > 0 {
			item.Share = float64(item.Tokens) / float64(totalTokens)
		}
		usage = append(usage, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate overview model usage: %w", err)
	}
	return usage, nil
}

func (s *postgresOverviewStore) LoadUsers(ctx context.Context, now time.Time) (usersResponse, error) {
	monthStart, monthEnd := monthWindow(now)
	const usersQuery = `
SELECT
  u.id::text AS user_id,
  u.email,
  u.display_name,
  COALESCE(SUM(utb.total_tokens), 0)::bigint AS used_tokens,
  u.status
FROM users u
LEFT JOIN usage_events ue
  ON ue.user_id = u.id
  AND ue.created_at >= $1
  AND ue.created_at < $2
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
GROUP BY u.id, u.email, u.display_name, u.status
ORDER BY used_tokens DESC, u.display_name ASC, u.email ASC`

	rows, err := s.db.QueryContext(ctx, usersQuery, monthStart, monthEnd)
	if err != nil {
		return usersResponse{}, fmt.Errorf("query admin users: %w", err)
	}
	defer rows.Close()

	response := emptyUsersResponse()
	for rows.Next() {
		var item adminUserUsage
		if err := rows.Scan(&item.UserID, &item.Email, &item.DisplayName, &item.UsedTokens, &item.Status); err != nil {
			return usersResponse{}, fmt.Errorf("scan admin users: %w", err)
		}
		response.Users = append(response.Users, item)
	}
	if err := rows.Err(); err != nil {
		return usersResponse{}, fmt.Errorf("iterate admin users: %w", err)
	}
	return response, nil
}

func (s *postgresOverviewStore) LoadModels(ctx context.Context, now time.Time) (modelsResponse, error) {
	monthStart, monthEnd := monthWindow(now)
	const modelsQuery = `
SELECT
  COALESCE(NULLIF(ue.model_actual, ''), ue.model_requested, 'unknown') AS model,
  COALESCE(NULLIF(ue.provider, ''), 'unknown') AS provider,
  COALESCE(SUM(utb.prompt_tokens), 0)::bigint AS prompt_tokens,
  COALESCE(SUM(utb.completion_tokens), 0)::bigint AS completion_tokens,
  COALESCE(SUM(utb.total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(cl.cost_usd), 0)::float8 AS cost_usd,
  COUNT(*)::bigint AS call_count
FROM usage_events ue
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
LEFT JOIN cost_ledger cl ON cl.usage_event_id = ue.id
WHERE ue.created_at >= $1 AND ue.created_at < $2
GROUP BY
  COALESCE(NULLIF(ue.model_actual, ''), ue.model_requested, 'unknown'),
  COALESCE(NULLIF(ue.provider, ''), 'unknown')
ORDER BY total_tokens DESC, model ASC`

	rows, err := s.db.QueryContext(ctx, modelsQuery, monthStart, monthEnd)
	if err != nil {
		return modelsResponse{}, fmt.Errorf("query admin models: %w", err)
	}
	defer rows.Close()

	response := emptyModelsResponse()
	for rows.Next() {
		var item adminModelUsage
		if err := rows.Scan(
			&item.Model,
			&item.Provider,
			&item.PromptTokens,
			&item.CompletionTokens,
			&item.TotalTokens,
			&item.CostUSD,
			&item.CallCount,
		); err != nil {
			return modelsResponse{}, fmt.Errorf("scan admin models: %w", err)
		}
		response.Models = append(response.Models, item)
	}
	if err := rows.Err(); err != nil {
		return modelsResponse{}, fmt.Errorf("iterate admin models: %w", err)
	}
	return response, nil
}

func makeCreateVirtualKeyHandler(logger *slog.Logger, bootstrapToken string, creator virtualKeyCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorizedBootstrap(r, bootstrapToken) {
			writeUnauthorized(w)
			return
		}

		var body createVirtualKeyRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope("invalid request body", "invalid_request", "invalid_json"))
			return
		}

		params, err := parseCreateVirtualKeyRequest(body)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope(err.Error(), "invalid_request", "invalid_parameters"))
			return
		}

		result, err := creator.CreateVirtualKey(r.Context(), params)
		if err != nil {
			logger.Error("create dev virtual key", "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to create virtual key", "server_error", "create_virtual_key_failed"))
			return
		}

		logger.Info("created dev virtual key",
			"organization_id", result.OrganizationID.String(),
			"user_id", result.UserID.String(),
			"key_prefix", result.KeyPrefix,
			"created_at", result.CreatedAt.UTC().Format(time.RFC3339),
		)

		httpx.WriteJSON(w, http.StatusCreated, createVirtualKeyResponse{
			DevOnly:        true,
			APIKeyID:       result.APIKeyID.String(),
			OrganizationID: result.OrganizationID.String(),
			UserID:         result.UserID.String(),
			KeyPrefix:      result.KeyPrefix,
			VirtualKey:     result.VirtualKey,
			CreatedAt:      result.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
}

func authorizedBootstrap(r *http.Request, bootstrapToken string) bool {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	token, ok := strings.CutPrefix(header, "Bearer ")
	// Demo-Ready 简化版: ordinary string comparison is enough for this dev-only
	// bootstrap path; full admin auth stays in T-005b.
	return ok && token == bootstrapToken
}

func parseCreateVirtualKeyRequest(body createVirtualKeyRequest) (createVirtualKeyParams, error) {
	orgID, err := uuid.Parse(strings.TrimSpace(body.OrganizationID))
	if err != nil {
		return createVirtualKeyParams{}, fmt.Errorf("organization_id must be a valid UUID")
	}
	userID, err := uuid.Parse(strings.TrimSpace(body.UserID))
	if err != nil {
		return createVirtualKeyParams{}, fmt.Errorf("user_id must be a valid UUID")
	}

	var projectID uuid.NullUUID
	if strings.TrimSpace(body.ProjectID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(body.ProjectID))
		if err != nil {
			return createVirtualKeyParams{}, fmt.Errorf("project_id must be a valid UUID")
		}
		projectID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	return createVirtualKeyParams{
		OrganizationID: orgID,
		UserID:         userID,
		ProjectID:      projectID,
		ExpiresAt:      body.ExpiresAt,
	}, nil
}

func (c *postgresVirtualKeyCreator) CreateVirtualKey(ctx context.Context, params createVirtualKeyParams) (createVirtualKeyResult, error) {
	random := c.random
	if random == nil {
		random = rand.Reader
	}
	now := c.now
	if now == nil {
		now = time.Now
	}

	key, err := auth.GenerateVirtualKeyFromReader(random)
	if err != nil {
		return createVirtualKeyResult{}, err
	}

	createdAt := now().UTC()
	var project any
	if params.ProjectID.Valid {
		project = params.ProjectID.UUID
	}
	var expires any
	if params.ExpiresAt != nil {
		expires = params.ExpiresAt.UTC()
	}

	const query = `
INSERT INTO api_keys (
  organization_id,
  project_id,
  user_id,
  key_prefix,
  key_hash,
  status,
  expires_at,
  created_at
)
VALUES ($1, $2, $3, $4, $5, 'active', $6, $7)
RETURNING id`

	var apiKeyID uuid.UUID
	if err := c.db.QueryRowContext(ctx, query,
		params.OrganizationID,
		project,
		params.UserID,
		key.Prefix,
		key.Hash,
		expires,
		createdAt,
	).Scan(&apiKeyID); err != nil {
		return createVirtualKeyResult{}, fmt.Errorf("insert virtual key: %w", err)
	}

	return createVirtualKeyResult{
		APIKeyID:       apiKeyID,
		OrganizationID: params.OrganizationID,
		UserID:         params.UserID,
		KeyPrefix:      key.Prefix,
		VirtualKey:     key.Token,
		CreatedAt:      createdAt,
	}, nil
}

func writeUnauthorized(w http.ResponseWriter) {
	httpx.WriteJSON(w, http.StatusUnauthorized, errorEnvelope("unauthorized", "authentication_error", "invalid_api_key"))
}

func errorEnvelope(message string, typ string, code string) map[string]any {
	return map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    typ,
			"code":    code,
		},
	}
}
