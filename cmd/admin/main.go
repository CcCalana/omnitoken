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
	"github.com/omnitoken/omnitoken/internal/credentials"
	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/rbac"
	usageanomaly "github.com/omnitoken/omnitoken/internal/usage/anomaly"
	"golang.org/x/crypto/bcrypt"
)

const (
	keyAnomalyThresholdEnv = "OMNITOKEN_ADMIN_KEY_ANOMALY_RPM_5M"
	adminSessionTTLEnv     = "OMNITOKEN_ADMIN_SESSION_TTL"
	defaultAdminSessionTTL = 24 * time.Hour
)

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

type virtualModelsResponse struct {
	VirtualModels []adminVirtualModel `json:"virtual_models"`
}

type credentialsResponse struct {
	Credentials []credentials.PublicCredential `json:"credentials"`
}

type adminVirtualModel struct {
	Name        string `json:"name"`
	RealModel   string `json:"real_model"`
	Status      string `json:"status"`
	Description string `json:"description"`
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

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
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
	LoadVirtualModels(context.Context) (virtualModelsResponse, error)
	LoadCredentials(context.Context) (credentialsResponse, error)
	Authenticate(ctx context.Context, email, password string) (uuid.UUID, uuid.UUID, string, error)
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

	sessionStore := auth.NewSessionStore(adminSessionTTL(logger))

	server := &http.Server{
		Addr:              cfg.Admin.Addr,
		Handler:           newMux(logger, cfg.Admin, overview, creator, sessionStore, auditRecorder),
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

func adminSessionTTL(logger *slog.Logger) time.Duration {
	raw := strings.TrimSpace(os.Getenv(adminSessionTTLEnv))
	if raw == "" {
		return defaultAdminSessionTTL
	}
	ttl, err := time.ParseDuration(raw)
	if err != nil || ttl <= 0 {
		logger.Warn("invalid admin session ttl",
			"env", adminSessionTTLEnv,
			"value", raw,
			"default", defaultAdminSessionTTL.String(),
		)
		return defaultAdminSessionTTL
	}
	return ttl
}

func newVirtualKeyCreator(logger *slog.Logger, _ config.Config, db *sql.DB) virtualKeyCreator {
	if db == nil {
		logger.Error("admin dev virtual key endpoint requires OMNITOKEN_DATABASE_URL")
		os.Exit(1)
	}

	return &postgresVirtualKeyCreator{db: db, random: nil, now: time.Now}
}

func newMux(logger *slog.Logger, cfg config.AdminConfig, overview overviewStore, creator virtualKeyCreator, sessionStore *auth.SessionStore, auditRecorders ...audit.Recorder) http.Handler {
	mux := http.NewServeMux()

	// Legacy dev-only bootstrap token authentication
	adminAuthBootstrap := adminAuthMiddleware(cfg.BootstrapToken)

	// New robust session-based authentication
	adminAuthSession := adminSessionMiddleware(sessionStore)

	var auditRecorder audit.Recorder
	if len(auditRecorders) > 0 {
		auditRecorder = auditRecorders[0]
	}
	adminAudit := audit.Middleware(auditRecorder, audit.MiddlewareConfig{
		ActorResolver: func(r *http.Request) audit.Actor {
			if subject, ok := auth.SubjectFromContext(r.Context()); ok {
				return audit.Actor{
					ID:   subject.UserID.String(),
					Type: "user",
				}
			}
			if cfg.BootstrapToken != "" && authorizedBootstrap(r, cfg.BootstrapToken) {
				return audit.BootstrapActorResolver(r)
			}
			return audit.Actor{
				ID:   "anonymous",
				Type: "anonymous",
			}
		},
		Logger: logger,
	})

	protectedWrite := func(action string, handler http.Handler) http.Handler {
		// Try session auth first, fallback to bootstrap (for tests/dev)
		return authChain(adminAuthSession, adminAuthBootstrap)(adminAudit(authorizeAdminWrite(cfg.BootstrapToken, action, handler)))
	}

	protectedRead := func(handler http.Handler) http.Handler {
		return authChain(adminAuthSession, adminAuthBootstrap)(handler)
	}

	adminAuditOnly := func(handler http.Handler) http.Handler {
		return adminAudit(handler)
	}

	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.Handle("POST /api/admin/login", adminAuditOnly(http.HandlerFunc(makeLoginHandler(logger, overview, sessionStore))))
	mux.Handle("POST /api/admin/logout", protectedRead(http.HandlerFunc(makeLogoutHandler(sessionStore))))
	mux.Handle("GET /api/admin/me", protectedRead(http.HandlerFunc(makeMeHandler())))

	mux.Handle("GET /api/admin/overview", protectedRead(http.HandlerFunc(makeOverviewHandler(logger, overview, time.Now))))
	mux.Handle("GET /api/admin/users", protectedRead(http.HandlerFunc(makeUsersHandler(logger, overview, time.Now))))
	mux.Handle("GET /api/admin/models", protectedRead(http.HandlerFunc(makeModelsHandler(logger, overview, time.Now))))
	mux.Handle("GET /api/admin/virtual-models", protectedRead(http.HandlerFunc(makeVirtualModelsHandler(logger, overview))))
	credentialsHandler := protectedRead(http.HandlerFunc(makeCredentialsHandler(logger, overview)))
	mux.Handle("GET /admin/credentials", credentialsHandler)
	mux.Handle("GET /api/admin/credentials", credentialsHandler)
	mux.Handle("GET /api/admin/audit-logs", protectedRead(http.HandlerFunc(makeAuditLogsHandler(logger, overview))))
	mux.Handle("PATCH /api/admin/users/{id}/quota", protectedWrite(rbac.ActionUpdateQuota, http.HandlerFunc(makeUpdateUserQuotaHandler(logger, overview))))
	if creator != nil {
		// Demo-Ready 简化版: this admin-port-only endpoint lives on
		// 8081, not the gateway data-plane port 8080. Session auth is primary;
		// bootstrap is only a local fallback when configured.
		mux.Handle("POST /api/admin/dev/virtual-keys", protectedWrite(rbac.ActionCreateVirtualKey, http.HandlerFunc(makeCreateVirtualKeyHandler(logger, creator))))
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

func makeVirtualModelsHandler(logger *slog.Logger, store overviewStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			httpx.WriteJSON(w, http.StatusOK, virtualModelsResponse{VirtualModels: []adminVirtualModel{}})
			return
		}

		models, err := store.LoadVirtualModels(r.Context())
		if err != nil {
			logger.Error("load virtual models", "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to load virtual models", "server_error", "virtual_models_query_failed"))
			return
		}

		httpx.WriteJSON(w, http.StatusOK, models)
	}
}

func makeCredentialsHandler(logger *slog.Logger, store overviewStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			httpx.WriteJSON(w, http.StatusOK, credentialsResponse{Credentials: []credentials.PublicCredential{}})
			return
		}
		response, err := store.LoadCredentials(r.Context())
		if err != nil {
			logger.Error("load admin credentials", "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to load credentials", "server_error", "load_credentials_failed"))
			return
		}
		if response.Credentials == nil {
			response.Credentials = []credentials.PublicCredential{}
		}
		httpx.WriteJSON(w, http.StatusOK, response)
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
  COALESCE(NULLIF(ue.model_routed, ''), NULLIF(ue.model_requested, ''), 'unknown') AS model,
  COALESCE(SUM(utb.total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(cl.cost_usd), 0)::float8 AS cost_usd
FROM usage_events ue
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
LEFT JOIN cost_ledger cl ON cl.usage_event_id = ue.id
WHERE ue.created_at >= $1 AND ue.created_at < $2
GROUP BY COALESCE(NULLIF(ue.model_routed, ''), NULLIF(ue.model_requested, ''), 'unknown')
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
  COALESCE(NULLIF(ue.model_routed, ''), NULLIF(ue.model_requested, ''), 'unknown') AS model,
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
  COALESCE(NULLIF(ue.model_routed, ''), NULLIF(ue.model_requested, ''), 'unknown'),
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

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled       = errors.New("user disabled")
)

func (s *postgresOverviewStore) Authenticate(ctx context.Context, email, password string) (uuid.UUID, uuid.UUID, string, error) {
	const query = `
SELECT
  u.id,
  u.organization_id,
  u.password_hash,
  u.status,
  COALESCE(
    (array_agg(r.canonical_name ORDER BY CASE r.canonical_name
      WHEN 'admin' THEN 1
      WHEN 'viewer' THEN 2
      WHEN 'member' THEN 3
      ELSE 4
    END))[1],
    ''
  ) AS role
FROM users u
LEFT JOIN role_assignments ra
  ON ra.organization_id = u.organization_id
 AND ra.user_id = u.id
LEFT JOIN roles r ON r.id = ra.role_id
WHERE u.email = $1
GROUP BY u.id, u.organization_id, u.password_hash, u.status`

	var userID, orgID uuid.UUID
	var hash sql.NullString
	var status string
	var role string
	if err := s.db.QueryRowContext(ctx, query, email).Scan(&userID, &orgID, &hash, &status, &role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, uuid.Nil, "", ErrInvalidCredentials
		}
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("query admin user: %w", err)
	}

	if status != "active" {
		return uuid.Nil, uuid.Nil, "", ErrUserDisabled
	}

	if !hash.Valid || strings.TrimSpace(role) == "" {
		return uuid.Nil, uuid.Nil, "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash.String), []byte(password)); err != nil {
		return uuid.Nil, uuid.Nil, "", ErrInvalidCredentials
	}

	return userID, orgID, role, nil
}

func (s *postgresOverviewStore) LoadVirtualModels(ctx context.Context) (virtualModelsResponse, error) {
	const query = `
SELECT name, real_model, status, COALESCE(description, '') as description
FROM virtual_models
ORDER BY name ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return virtualModelsResponse{}, fmt.Errorf("query virtual models: %w", err)
	}
	defer rows.Close()

	response := virtualModelsResponse{VirtualModels: []adminVirtualModel{}}
	for rows.Next() {
		var item adminVirtualModel
		if err := rows.Scan(&item.Name, &item.RealModel, &item.Status, &item.Description); err != nil {
			return virtualModelsResponse{}, fmt.Errorf("scan virtual models: %w", err)
		}
		response.VirtualModels = append(response.VirtualModels, item)
	}
	if err := rows.Err(); err != nil {
		return virtualModelsResponse{}, fmt.Errorf("iterate virtual models: %w", err)
	}
	return response, nil
}

func (s *postgresOverviewStore) LoadCredentials(ctx context.Context) (credentialsResponse, error) {
	items, err := credentials.NewPostgresStore(s.db, nil).ListPublic(ctx)
	if err != nil {
		return credentialsResponse{}, err
	}
	if items == nil {
		items = []credentials.PublicCredential{}
	}
	return credentialsResponse{Credentials: items}, nil
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
			if bootstrapToken == "" || !authorizedBootstrap(r, bootstrapToken) {
				// In the session-primary admin architecture, an empty bootstrap
				// token means the dev fallback is disabled, not that admin auth is
				// globally unconfigured.
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func authorizeAdminWrite(bootstrapToken string, action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subject, ok := auth.SubjectFromContext(r.Context()); ok {
			if subject.Role == string(rbac.RoleAdmin) {
				next.ServeHTTP(w, r)
				return
			}
			audit.SetAction(r.Context(), action)
			audit.SetAfter(r.Context(), map[string]string{"reason": rbac.ReasonActionNotPermitted})
			httpx.WriteJSON(w, http.StatusForbidden, errorEnvelope("forbidden", "authorization_error", "rbac_denied"))
			return
		}
		if strings.TrimSpace(bootstrapToken) != "" && authorizedBootstrap(r, bootstrapToken) {
			next.ServeHTTP(w, r)
			return
		}
		writeUnauthorized(w)
	})
}

type authResponseWriter struct {
	http.ResponseWriter
	unauthorized bool
	wroteHeader  bool
}

func (w *authResponseWriter) WriteHeader(code int) {
	if code == http.StatusUnauthorized {
		w.unauthorized = true
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *authResponseWriter) Write(b []byte) (int, error) {
	if w.unauthorized {
		return len(b), nil
	}
	if !w.wroteHeader {
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func authChain(primary, fallback func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pw := &authResponseWriter{ResponseWriter: w}

			passed := false
			primaryHandler := primary(http.HandlerFunc(func(innerW http.ResponseWriter, innerR *http.Request) {
				passed = true
				next.ServeHTTP(w, innerR)
			}))

			primaryHandler.ServeHTTP(pw, r)

			if !passed && pw.unauthorized {
				fallback(next).ServeHTTP(w, r)
			}
		})
	}
}

func adminSessionMiddleware(store *auth.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				writeUnauthorized(w)
				return
			}
			token := extractBearerToken(r)
			if token == "" {
				writeUnauthorized(w)
				return
			}
			session, ok := store.Validate(token)
			if !ok {
				writeUnauthorized(w)
				return
			}
			ctx := auth.WithSubject(r.Context(), auth.Subject{
				UserID: session.UserID,
				OrgID:  session.OrgID,
				Role:   session.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func makeLoginHandler(logger *slog.Logger, store overviewStore, sessionStore *auth.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessionStore == nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, errorEnvelope("admin session store not configured", "server_error", "session_store_not_configured"))
			return
		}
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope("invalid request", "invalid_request", "invalid_json"))
			return
		}

		if req.Email == "" || req.Password == "" {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope("email and password required", "invalid_request", "missing_credentials"))
			return
		}

		userID, orgID, role, err := store.Authenticate(r.Context(), req.Email, req.Password)
		if err != nil {
			// Record failed login audit log
			audit.SetAction(r.Context(), "login_failed")
			audit.SetAfter(r.Context(), map[string]string{"email": req.Email, "reason": err.Error()})
			logger.Warn("admin login failed", "email", req.Email, "error", err)

			if errors.Is(err, ErrInvalidCredentials) {
				httpx.WriteJSON(w, http.StatusUnauthorized, errorEnvelope("invalid credentials", "authentication_error", "invalid_credentials"))
			} else if errors.Is(err, ErrUserDisabled) {
				httpx.WriteJSON(w, http.StatusForbidden, errorEnvelope("user disabled", "authentication_error", "user_disabled"))
			} else {
				httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("internal error", "server_error", "internal_error"))
			}
			return
		}

		token, err := sessionStore.Create(userID, orgID, role)
		if err != nil {
			logger.Error("create session failed", "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("internal error", "server_error", "internal_error"))
			return
		}

		// Record successful login audit log
		audit.SetAction(r.Context(), "login")
		audit.SetAfter(r.Context(), map[string]string{"user_id": userID.String()})
		logger.Info("admin login success", "user_id", userID.String(), "email", req.Email)

		httpx.WriteJSON(w, http.StatusOK, loginResponse{Token: token})
	}
}

func makeLogoutHandler(sessionStore *auth.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessionStore == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		token := extractBearerToken(r)
		if token != "" {
			sessionStore.Revoke(token)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func makeMeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subject, ok := auth.SubjectFromContext(r.Context())
		if !ok {
			writeUnauthorized(w)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{
			"user_id": subject.UserID.String(),
			"role":    subject.Role,
		})
	}
}

func extractBearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(token)
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
