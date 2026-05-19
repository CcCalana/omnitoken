package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/omnitoken/omnitoken/internal/audit"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/rbac"
	usageanomaly "github.com/omnitoken/omnitoken/internal/usage/anomaly"
)

const keyAnomalyThresholdEnv = "OMNITOKEN_ADMIN_KEY_ANOMALY_RPM_5M"

var errAdminUserNotFound = errors.New("admin user not found")

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
	UserID          string `json:"user_id"`
	Email           string `json:"email"`
	DisplayName     string `json:"display_name"`
	UsedTokens      int64  `json:"used_tokens"`
	UsedBudgetCents int64  `json:"used_budget_cents"`
	BudgetCents     *int64 `json:"budget_cents"`
	Quota           int64  `json:"quota"`
	Status          string `json:"status"`
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

type auditLogFilters struct {
	ActorID      string
	ResourceType string
	ResourceID   string
	Since        *time.Time
	Until        *time.Time
	Limit        int
}

type adminAuditLog struct {
	ActorID      string          `json:"actor_id"`
	ActorType    string          `json:"actor_type"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   *string         `json:"resource_id"`
	Before       json.RawMessage `json:"before"`
	After        json.RawMessage `json:"after"`
	IP           *string         `json:"ip"`
	UserAgent    string          `json:"user_agent"`
	RequestID    string          `json:"request_id"`
	StatusCode   int             `json:"status_code"`
	CreatedAt    string          `json:"created_at"`
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

type updateUserQuotaResponse struct {
	UserID      string `json:"user_id"`
	BudgetCents *int64 `json:"budget_cents"`
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

type updateUserBudgetResult struct {
	BeforeBudgetCents *int64
	AfterBudgetCents  *int64
}

type virtualKeyCreator interface {
	CreateVirtualKey(context.Context, createVirtualKeyParams) (createVirtualKeyResult, error)
}

type overviewStore interface {
	LoadOverview(context.Context, time.Time) (overviewResponse, error)
	LoadUsers(context.Context, time.Time) (usersResponse, error)
	LoadModels(context.Context, time.Time) (modelsResponse, error)
	LoadAuditLogs(context.Context, auditLogFilters) ([]adminAuditLog, error)
	UpdateUserBudget(context.Context, uuid.UUID, *int64, time.Time) (updateUserBudgetResult, error)
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
	auditRecorder := newAuditRecorder(logger, db)
	runCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()
	startKeyAnomalyMonitor(runCtx, logger, db)

	server := &http.Server{
		Addr:              cfg.Admin.Addr,
		Handler:           newMux(logger, cfg.Admin, overview, creator, auditRecorder),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("admin listening", "addr", cfg.Admin.Addr)
	if err := httpx.Run(runCtx, server); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

func newAuditRecorder(logger *slog.Logger, db *sql.DB) audit.Recorder {
	if db == nil {
		return audit.NewRecorder(nil, logger)
	}
	return audit.NewRecorder(audit.NewPostgresStore(db), logger)
}

func startKeyAnomalyMonitor(ctx context.Context, logger *slog.Logger, db *sql.DB) {
	if db == nil {
		return
	}
	monitor := usageanomaly.NewMonitor(
		usageanomaly.NewPostgresStore(db),
		usageanomaly.Config{
			Threshold: keyAnomalyThreshold(logger),
			Logger:    logger,
		},
	)
	monitor.Start(ctx)
}

func keyAnomalyThreshold(logger *slog.Logger) int {
	raw := strings.TrimSpace(os.Getenv(keyAnomalyThresholdEnv))
	if raw == "" {
		return usageanomaly.DefaultThreshold
	}
	threshold, err := strconv.Atoi(raw)
	if err != nil || threshold <= 0 {
		logger.Warn("invalid key anomaly threshold",
			"env", keyAnomalyThresholdEnv,
			"value", raw,
			"default", usageanomaly.DefaultThreshold,
		)
		return usageanomaly.DefaultThreshold
	}
	return threshold
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

func newMux(logger *slog.Logger, cfg config.AdminConfig, overview overviewStore, creator virtualKeyCreator, auditRecorders ...audit.Recorder) http.Handler {
	mux := http.NewServeMux()
	adminAuth := adminAuthMiddleware(cfg.BootstrapToken)
	var auditRecorder audit.Recorder
	if len(auditRecorders) > 0 {
		auditRecorder = auditRecorders[0]
	}
	adminAudit := audit.Middleware(auditRecorder, audit.MiddlewareConfig{
		ActorResolver: audit.BootstrapActorResolver,
		Logger:        logger,
	})
	protectedWrite := func(handler http.Handler) http.Handler {
		return adminAuth(adminAudit(handler))
	}

	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.Handle("GET /api/admin/overview", adminAuth(http.HandlerFunc(makeOverviewHandler(logger, overview, time.Now))))
	mux.Handle("GET /api/admin/users", adminAuth(http.HandlerFunc(makeUsersHandler(logger, overview, time.Now))))
	mux.Handle("GET /api/admin/models", adminAuth(http.HandlerFunc(makeModelsHandler(logger, overview, time.Now))))
	mux.Handle("GET /api/admin/audit-logs", adminAuth(http.HandlerFunc(makeAuditLogsHandler(logger, overview))))
	mux.Handle("PATCH /api/admin/users/{id}/quota", protectedWrite(http.HandlerFunc(makeUpdateUserQuotaHandler(logger, overview))))
	if cfg.BootstrapToken != "" && creator != nil {
		// Demo-Ready 简化版: this admin-port-only endpoint lives on
		// 8081, not the gateway data-plane port 8080. Full admin auth/RBAC stays
		// in T-005b.
		mux.Handle("POST /api/admin/dev/virtual-keys", protectedWrite(http.HandlerFunc(makeCreateVirtualKeyHandler(logger, creator))))
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

func makeAuditLogsHandler(logger *slog.Logger, store overviewStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filters, err := parseAuditLogFilters(r)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope(err.Error(), "invalid_request", "invalid_audit_log_filters"))
			return
		}
		if store == nil {
			httpx.WriteJSON(w, http.StatusOK, []adminAuditLog{})
			return
		}

		logs, err := store.LoadAuditLogs(r.Context(), filters)
		if err != nil {
			logger.Error("load audit logs", "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to load audit logs", "server_error", "audit_logs_query_failed"))
			return
		}

		httpx.WriteJSON(w, http.StatusOK, logs)
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

func parseAuditLogFilters(r *http.Request) (auditLogFilters, error) {
	query := r.URL.Query()
	filters := auditLogFilters{
		ActorID:      strings.TrimSpace(query.Get("actor_id")),
		ResourceType: strings.TrimSpace(query.Get("resource_type")),
		ResourceID:   strings.TrimSpace(query.Get("resource_id")),
		Limit:        100,
	}

	if value := strings.TrimSpace(query.Get("limit")); value != "" {
		limit, err := parsePositiveInt(value)
		if err != nil {
			return auditLogFilters{}, fmt.Errorf("invalid limit")
		}
		if limit > 500 {
			limit = 500
		}
		filters.Limit = limit
	}

	since, err := parseOptionalRFC3339(query.Get("since"))
	if err != nil {
		return auditLogFilters{}, fmt.Errorf("invalid since")
	}
	until, err := parseOptionalRFC3339(query.Get("until"))
	if err != nil {
		return auditLogFilters{}, fmt.Errorf("invalid until")
	}
	filters.Since = since
	filters.Until = until
	return filters, nil
}

func parseOptionalRFC3339(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}

func parsePositiveInt(value string) (int, error) {
	out, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if out <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return out, nil
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
  -- Display value only: round sub-cent cost upward so UI never understates use.
  -- Gateway enforcement compares exact cost_usd against budget cents.
  COALESCE(CEIL(COALESCE(SUM(cl.cost_usd), 0) * 100), 0)::bigint AS used_budget_cents,
  u.monthly_budget_cents,
  u.status
FROM users u
LEFT JOIN usage_events ue
  ON ue.user_id = u.id
  AND ue.created_at >= $1
  AND ue.created_at < $2
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
LEFT JOIN cost_ledger cl ON cl.usage_event_id = ue.id
GROUP BY u.id, u.email, u.display_name, u.monthly_budget_cents, u.status
ORDER BY used_tokens DESC, u.display_name ASC, u.email ASC`

	rows, err := s.db.QueryContext(ctx, usersQuery, monthStart, monthEnd)
	if err != nil {
		return usersResponse{}, fmt.Errorf("query admin users: %w", err)
	}
	defer rows.Close()

	response := emptyUsersResponse()
	for rows.Next() {
		var item adminUserUsage
		var budgetCents sql.NullInt64
		if err := rows.Scan(&item.UserID, &item.Email, &item.DisplayName, &item.UsedTokens, &item.UsedBudgetCents, &budgetCents, &item.Status); err != nil {
			return usersResponse{}, fmt.Errorf("scan admin users: %w", err)
		}
		item.BudgetCents = nullableInt64Ptr(budgetCents)
		if item.BudgetCents != nil {
			item.Quota = *item.BudgetCents
		}
		response.Users = append(response.Users, item)
	}
	if err := rows.Err(); err != nil {
		return usersResponse{}, fmt.Errorf("iterate admin users: %w", err)
	}
	return response, nil
}

func (s *postgresOverviewStore) UpdateUserBudget(ctx context.Context, userID uuid.UUID, budgetCents *int64, updatedAt time.Time) (updateUserBudgetResult, error) {
	var budget any
	if budgetCents != nil {
		budget = *budgetCents
	}

	const updateQuery = `
WITH target AS (
  SELECT id, monthly_budget_cents
  FROM users
  WHERE id = $1
),
updated AS (
  UPDATE users
  SET monthly_budget_cents = $2,
      updated_at = $3
  WHERE id = $1
  RETURNING id, monthly_budget_cents
)
SELECT
  target.monthly_budget_cents AS before_budget_cents,
  updated.monthly_budget_cents AS after_budget_cents
FROM target
JOIN updated ON updated.id = target.id`

	var before sql.NullInt64
	var after sql.NullInt64
	if err := s.db.QueryRowContext(ctx, updateQuery, userID, budget, updatedAt.UTC()).Scan(&before, &after); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return updateUserBudgetResult{}, errAdminUserNotFound
		}
		return updateUserBudgetResult{}, fmt.Errorf("update user budget: %w", err)
	}
	return updateUserBudgetResult{
		BeforeBudgetCents: nullableInt64Ptr(before),
		AfterBudgetCents:  nullableInt64Ptr(after),
	}, nil
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

func (s *postgresOverviewStore) LoadAuditLogs(ctx context.Context, filters auditLogFilters) ([]adminAuditLog, error) {
	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Limit > 500 {
		filters.Limit = 500
	}

	query, args := buildAuditLogsQuery(filters)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	logs := []adminAuditLog{}
	for rows.Next() {
		item, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}
	return logs, nil
}

type auditLogScanner interface {
	Scan(dest ...any) error
}

func scanAuditLog(scanner auditLogScanner) (adminAuditLog, error) {
	var item adminAuditLog
	var resourceID sql.NullString
	var before sql.NullString
	var after sql.NullString
	var ip sql.NullString
	var createdAt time.Time
	if err := scanner.Scan(
		&item.ActorID,
		&item.ActorType,
		&item.Action,
		&item.ResourceType,
		&resourceID,
		&before,
		&after,
		&ip,
		&item.UserAgent,
		&item.RequestID,
		&item.StatusCode,
		&createdAt,
	); err != nil {
		return adminAuditLog{}, fmt.Errorf("scan audit log: %w", err)
	}
	item.ResourceID = nullableStringPtr(resourceID)
	item.Before = nullableRawJSON(before)
	item.After = nullableRawJSON(after)
	item.IP = nullableStringPtr(ip)
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return item, nil
}

func buildAuditLogsQuery(filters auditLogFilters) (string, []any) {
	var builder strings.Builder
	builder.WriteString(`
SELECT
  actor_id,
  actor_type,
  action,
  resource_type,
  resource_id,
  "before"::text,
  "after"::text,
  ip::text,
  user_agent,
  request_id,
  status_code,
  created_at
FROM audit_logs`)

	args := make([]any, 0, 6)
	conditions := make([]string, 0, 5)
	addCondition := func(sql string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(sql, len(args)))
	}
	if filters.ActorID != "" {
		addCondition("actor_id = $%d", filters.ActorID)
	}
	if filters.ResourceType != "" {
		addCondition("resource_type = $%d", filters.ResourceType)
	}
	if filters.ResourceID != "" {
		addCondition("resource_id = $%d", filters.ResourceID)
	}
	if filters.Since != nil {
		addCondition("created_at >= $%d", filters.Since.UTC())
	}
	if filters.Until != nil {
		addCondition("created_at < $%d", filters.Until.UTC())
	}
	if len(conditions) > 0 {
		builder.WriteString("\nWHERE ")
		builder.WriteString(strings.Join(conditions, "\n  AND "))
	}

	args = append(args, filters.Limit)
	builder.WriteString(fmt.Sprintf("\nORDER BY created_at DESC, id DESC\nLIMIT $%d", len(args)))
	return builder.String(), args
}

func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	out := value.String
	return &out
}

func nullableInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}

func nullableRawJSON(value sql.NullString) json.RawMessage {
	if !value.Valid {
		return nil
	}
	return json.RawMessage(value.String)
}

func makeUpdateUserQuotaHandler(logger *slog.Logger, store overviewStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope("user id must be a valid UUID", "invalid_request", "invalid_user_id"))
			return
		}

		audit.SetAction(r.Context(), rbac.ActionUpdateQuota)
		audit.SetResource(r.Context(), "user_quota", userID.String())

		budgetCents, err := parseUpdateUserQuotaRequest(w, r)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope(err.Error(), "invalid_request", "invalid_quota"))
			return
		}
		if store == nil {
			logger.Error("update user quota without admin store", "user_id", userID.String())
			httpx.WriteJSON(w, http.StatusServiceUnavailable, errorEnvelope("admin store is not configured", "server_error", "admin_store_not_configured"))
			return
		}

		result, err := store.UpdateUserBudget(r.Context(), userID, budgetCents, time.Now().UTC())
		if err != nil {
			if errors.Is(err, errAdminUserNotFound) {
				httpx.WriteJSON(w, http.StatusNotFound, errorEnvelope("user not found", "invalid_request", "user_not_found"))
				return
			}
			logger.Error("update user quota", "user_id", userID.String(), "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to update quota", "server_error", "update_quota_failed"))
			return
		}

		audit.SetBefore(r.Context(), map[string]any{"budget_cents": result.BeforeBudgetCents})
		audit.SetAfter(r.Context(), map[string]any{"budget_cents": result.AfterBudgetCents})

		httpx.WriteJSON(w, http.StatusOK, updateUserQuotaResponse{
			UserID:      userID.String(),
			BudgetCents: result.AfterBudgetCents,
		})
	}
}

func parseUpdateUserQuotaRequest(w http.ResponseWriter, r *http.Request) (*int64, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}
	value, ok := raw["budget_cents"]
	if !ok {
		return nil, fmt.Errorf("budget_cents is required")
	}
	if strings.TrimSpace(string(value)) == "null" {
		return nil, nil
	}
	var budget int64
	if err := json.Unmarshal(value, &budget); err != nil {
		return nil, fmt.Errorf("budget_cents must be an integer")
	}
	if budget < 0 {
		return nil, fmt.Errorf("budget_cents must be non-negative")
	}
	return &budget, nil
}

func makeCreateVirtualKeyHandler(logger *slog.Logger, creator virtualKeyCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		audit.SetAction(r.Context(), "create_virtual_key")
		audit.SetResource(r.Context(), "virtual_key", result.APIKeyID.String())
		audit.SetBefore(r.Context(), nil)
		audit.SetAfter(r.Context(), map[string]string{
			"api_key_id":      result.APIKeyID.String(),
			"organization_id": result.OrganizationID.String(),
			"user_id":         result.UserID.String(),
			"key_prefix":      result.KeyPrefix,
		})

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

func adminAuthMiddleware(bootstrapToken string) func(http.Handler) http.Handler {
	bootstrapToken = strings.TrimSpace(bootstrapToken)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bootstrapToken == "" {
				writeAdminAuthNotConfigured(w)
				return
			}
			if !authorizedBootstrap(r, bootstrapToken) {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func authorizedBootstrap(r *http.Request, bootstrapToken string) bool {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return false
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) != len(bootstrapToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(bootstrapToken)) == 1
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

func writeAdminAuthNotConfigured(w http.ResponseWriter) {
	httpx.WriteJSON(w, http.StatusServiceUnavailable, errorEnvelope("admin auth is not configured", "server_error", "admin_auth_not_configured"))
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
