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
	"net/url"
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
	omnicrypto "github.com/omnitoken/omnitoken/internal/crypto"
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

type createCredentialRequest struct {
	Provider string `json:"provider"`
	Alias    string `json:"alias"`
	Priority int    `json:"priority"`
	BaseURL  string `json:"base_url"`
	Key      string `json:"key"`
}

type adminVirtualModel struct {
	Name        string `json:"name"`
	RealModel   string `json:"real_model"`
	Provider    string `json:"provider"`
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

type userUsageFilters struct {
	Since *time.Time
	Until *time.Time
	TopN  int
}

type userUsagePeriod struct {
	Name  string `json:"name"`
	Since string `json:"since"`
	Until string `json:"until"`
}

type userUsageResponse struct {
	UserID             string                `json:"user_id"`
	Period             userUsagePeriod       `json:"period"`
	ModelTop           []userUsageModelTop   `json:"model_top"`
	HourlyDistribution []int64               `json:"hourly_distribution"`
	RecentCalls        []userUsageRecentCall `json:"recent_calls"`
}

type userUsageModelTop struct {
	Model     string `json:"model"`
	Tokens    int64  `json:"tokens"`
	CallCount int64  `json:"call_count"`
}

type userUsageRecentCall struct {
	CreatedAt   string `json:"created_at"`
	Model       string `json:"model"`
	StatusCode  int    `json:"status_code"`
	TotalTokens int64  `json:"total_tokens"`
	Streaming   bool   `json:"streaming"`
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
	LoadUserUsage(context.Context, uuid.UUID, userUsageFilters, time.Time) (userUsageResponse, error)
	UpdateUserBudget(context.Context, uuid.UUID, *int64, time.Time) (updateUserBudgetResult, error)
	LoadVirtualModels(context.Context) (virtualModelsResponse, error)
	LoadCredentials(context.Context) (credentialsResponse, error)
	CreateCredential(context.Context, credentials.CreateParams) (credentials.PublicCredential, error)
	DisableCredential(context.Context, string) (credentials.PublicCredential, error)
	Authenticate(ctx context.Context, email, password string) (uuid.UUID, uuid.UUID, string, error)
}

type postgresVirtualKeyCreator struct {
	db     *sql.DB
	random io.Reader
	now    func() time.Time
}

type postgresOverviewStore struct {
	db          *sql.DB
	envelope    *omnicrypto.Envelope
	envelopeErr error
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
	overview := newOverviewStore(logger, cfg, db)
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

	db, err := sql.Open("postgres", postgresURLWithApplicationName(cfg.DatabaseURL, "omnitoken-admin"))
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

func postgresURLWithApplicationName(databaseURL, applicationName string) string {
	dsn := strings.TrimSpace(databaseURL)
	name := strings.TrimSpace(applicationName)
	if dsn == "" || name == "" {
		return databaseURL
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		return dsn + separator + "application_name=" + name
	}
	return dsn + " application_name=" + name
}

func newOverviewStore(logger *slog.Logger, cfg config.Config, db *sql.DB) overviewStore {
	if db == nil {
		return nil
	}
	store := &postgresOverviewStore{db: db}
	masterKey, err := omnicrypto.LoadMasterKey(cfg.MasterKeyFile, cfg.MasterKey)
	if err != nil {
		store.envelopeErr = err
		return store
	}
	envelope, err := omnicrypto.NewEnvelope(masterKey)
	if err != nil {
		store.envelopeErr = err
		if logger != nil {
			logger.Warn("admin credential encryption disabled", "err", err)
		}
		return store
	}
	store.envelope = envelope
	return store
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
	createCredentialHandler := protectedWrite(rbac.ActionCreateCredential, http.HandlerFunc(makeCreateCredentialHandler(logger, overview)))
	mux.Handle("POST /admin/credentials", createCredentialHandler)
	mux.Handle("POST /api/admin/credentials", createCredentialHandler)
	disableCredentialHandler := protectedWrite(rbac.ActionDisableCredential, http.HandlerFunc(makeDisableCredentialHandler(logger, overview)))
	mux.Handle("PATCH /admin/credentials/{id}/disable", disableCredentialHandler)
	mux.Handle("PATCH /api/admin/credentials/{id}/disable", disableCredentialHandler)
	mux.Handle("GET /api/admin/audit-logs", protectedRead(http.HandlerFunc(makeAuditLogsHandler(logger, overview))))
	mux.Handle("GET /api/admin/users/{id}/usage", protectedRead(http.HandlerFunc(makeUserUsageHandler(logger, overview, time.Now))))
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

func makeCreateCredentialHandler(logger *slog.Logger, store overviewStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		audit.SetAction(r.Context(), rbac.ActionCreateCredential)
		audit.SetResource(r.Context(), "upstream_credential", "")

		var body createCredentialRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope("invalid request body", "invalid_request", "invalid_json"))
			return
		}
		params, err := parseCreateCredentialRequest(body)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope(err.Error(), "invalid_request", "invalid_credential"))
			return
		}
		audit.SetBefore(r.Context(), nil)
		audit.SetAfter(r.Context(), credentialAuditView(params.Provider, params.Alias, params.Priority, params.BaseURL, credentials.StatusActive, credentials.HealthHealthy))

		if store == nil {
			logger.Error("create credential without admin store")
			httpx.WriteJSON(w, http.StatusServiceUnavailable, errorEnvelope("admin store is not configured", "server_error", "admin_store_not_configured"))
			return
		}
		created, err := store.CreateCredential(r.Context(), params)
		if err != nil {
			if errors.Is(err, omnicrypto.ErrMasterKeyMissing) {
				httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("master key is required to encrypt credentials", "server_error", "master_key_missing"))
				return
			}
			if errors.Is(err, credentials.ErrAliasExists) {
				httpx.WriteJSON(w, http.StatusConflict, errorEnvelope("alias already exists for provider", "invalid_request", "credential_alias_exists"))
				return
			}
			logger.Error("create credential", "provider", params.Provider, "alias", params.Alias, "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to create credential", "server_error", "create_credential_failed"))
			return
		}
		audit.SetResource(r.Context(), "upstream_credential", created.ID)
		audit.SetAfter(r.Context(), credentialAuditView(created.Provider, aliasFromMetadata(created.Metadata), created.Priority, created.BaseURL, created.Status, created.HealthState))
		httpx.WriteJSON(w, http.StatusCreated, created)
	}
}

func makeDisableCredentialHandler(logger *slog.Logger, store overviewStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("id"))
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope("credential id must be a valid UUID", "invalid_request", "invalid_credential_id"))
			return
		}
		audit.SetAction(r.Context(), rbac.ActionDisableCredential)
		audit.SetResource(r.Context(), "upstream_credential", id)

		if store == nil {
			logger.Error("disable credential without admin store", "credential_id", id)
			httpx.WriteJSON(w, http.StatusServiceUnavailable, errorEnvelope("admin store is not configured", "server_error", "admin_store_not_configured"))
			return
		}
		disabled, err := store.DisableCredential(r.Context(), id)
		if err != nil {
			if errors.Is(err, credentials.ErrCredentialMissing) {
				httpx.WriteJSON(w, http.StatusNotFound, errorEnvelope("credential not found", "invalid_request", "credential_not_found"))
				return
			}
			logger.Error("disable credential", "credential_id", id, "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to disable credential", "server_error", "disable_credential_failed"))
			return
		}
		audit.SetAfter(r.Context(), credentialAuditView(disabled.Provider, aliasFromMetadata(disabled.Metadata), disabled.Priority, disabled.BaseURL, disabled.Status, disabled.HealthState))
		httpx.WriteJSON(w, http.StatusOK, disabled)
	}
}

func parseCreateCredentialRequest(body createCredentialRequest) (credentials.CreateParams, error) {
	provider := strings.TrimSpace(body.Provider)
	switch provider {
	case "ark", "deepseek":
	default:
		return credentials.CreateParams{}, fmt.Errorf("provider must be ark or deepseek")
	}
	alias := strings.TrimSpace(body.Alias)
	if alias == "" {
		return credentials.CreateParams{}, fmt.Errorf("alias is required")
	}
	baseURL := strings.TrimSpace(body.BaseURL)
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return credentials.CreateParams{}, fmt.Errorf("base_url must be http(s)://")
	}
	if body.Priority < 1 {
		return credentials.CreateParams{}, fmt.Errorf("priority must be at least 1")
	}
	key := strings.TrimSpace(body.Key)
	if key == "" {
		return credentials.CreateParams{}, fmt.Errorf("key is required")
	}
	return credentials.CreateParams{
		Provider: provider,
		Alias:    alias,
		BaseURL:  baseURL,
		Priority: body.Priority,
		Secret:   key,
	}, nil
}

func credentialAuditView(provider, alias string, priority int, baseURL string, status string, healthState string) map[string]any {
	return map[string]any{
		"provider":     provider,
		"alias":        alias,
		"priority":     priority,
		"base_url":     baseURL,
		"status":       status,
		"health_state": healthState,
	}
}

func aliasFromMetadata(metadata json.RawMessage) string {
	if len(metadata) == 0 {
		return ""
	}
	var body struct {
		Alias string `json:"alias"`
	}
	if err := json.Unmarshal(metadata, &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.Alias)
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

func makeUserUsageHandler(logger *slog.Logger, store overviewStore, now func() time.Time) http.HandlerFunc {
	if now == nil {
		now = time.Now
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope("user id must be a valid UUID", "invalid_request", "invalid_user_id"))
			return
		}
		filters, err := parseUserUsageFilters(r)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope(err.Error(), "invalid_request", "invalid_user_usage_filters"))
			return
		}
		if store == nil {
			httpx.WriteJSON(w, http.StatusOK, emptyUserUsageResponse(userID, filters, now()))
			return
		}

		usage, err := store.LoadUserUsage(r.Context(), userID, filters, now())
		if err != nil {
			logger.Error("load user usage", "user_id", userID.String(), "error", err)
			httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope("failed to load user usage", "server_error", "user_usage_query_failed"))
			return
		}

		httpx.WriteJSON(w, http.StatusOK, usage)
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

func emptyUserUsageResponse(userID uuid.UUID, filters userUsageFilters, now time.Time) userUsageResponse {
	since, until, name := userUsageWindow(filters, now)
	return userUsageResponse{
		UserID: userID.String(),
		Period: userUsagePeriod{
			Name:  name,
			Since: since.Format(time.RFC3339),
			Until: until.Format(time.RFC3339),
		},
		ModelTop:           []userUsageModelTop{},
		HourlyDistribution: make([]int64, 24),
		RecentCalls:        []userUsageRecentCall{},
	}
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

func parseUserUsageFilters(r *http.Request) (userUsageFilters, error) {
	query := r.URL.Query()
	filters := userUsageFilters{TopN: 10}
	if raw := strings.TrimSpace(query.Get("top_n")); raw != "" {
		topN, err := parsePositiveInt(raw)
		if err != nil {
			return userUsageFilters{}, fmt.Errorf("invalid top_n")
		}
		if topN > 50 {
			topN = 50
		}
		filters.TopN = topN
	}
	if raw := strings.TrimSpace(query.Get("since")); raw != "" {
		since, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return userUsageFilters{}, fmt.Errorf("invalid since")
		}
		since = since.UTC()
		filters.Since = &since
	}
	if raw := strings.TrimSpace(query.Get("until")); raw != "" {
		until, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return userUsageFilters{}, fmt.Errorf("invalid until")
		}
		until = until.UTC()
		filters.Until = &until
	}
	if filters.Since != nil && filters.Until != nil && !filters.Until.After(*filters.Since) {
		return userUsageFilters{}, fmt.Errorf("until must be after since")
	}
	return filters, nil
}

func userUsageWindow(filters userUsageFilters, now time.Time) (time.Time, time.Time, string) {
	monthStart, monthEnd := monthWindow(now)
	since := monthStart
	until := monthEnd
	name := "current_month"
	if filters.Since != nil {
		since = filters.Since.UTC()
		name = "custom"
	}
	if filters.Until != nil {
		until = filters.Until.UTC()
		name = "custom"
	}
	return since, until, name
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

func (s *postgresOverviewStore) LoadUserUsage(ctx context.Context, userID uuid.UUID, filters userUsageFilters, now time.Time) (userUsageResponse, error) {
	since, until, name := userUsageWindow(filters, now)
	response := userUsageResponse{
		UserID: userID.String(),
		Period: userUsagePeriod{
			Name:  name,
			Since: since.Format(time.RFC3339),
			Until: until.Format(time.RFC3339),
		},
		ModelTop:           []userUsageModelTop{},
		HourlyDistribution: make([]int64, 24),
		RecentCalls:        []userUsageRecentCall{},
	}

	modelTop, err := s.loadUserUsageModelTop(ctx, userID, since, until, filters.TopN)
	if err != nil {
		return userUsageResponse{}, err
	}
	response.ModelTop = modelTop

	hourly, err := s.loadUserUsageHourly(ctx, userID, since, until)
	if err != nil {
		return userUsageResponse{}, err
	}
	response.HourlyDistribution = hourly

	recent, err := s.loadUserUsageRecentCalls(ctx, userID, since, until)
	if err != nil {
		return userUsageResponse{}, err
	}
	response.RecentCalls = recent

	return response, nil
}

func (s *postgresOverviewStore) loadUserUsageModelTop(ctx context.Context, userID uuid.UUID, since time.Time, until time.Time, topN int) ([]userUsageModelTop, error) {
	if topN <= 0 {
		topN = 10
	}
	const query = `
SELECT
  COALESCE(NULLIF(ue.model_routed, ''), 'unknown') AS model,
  COALESCE(SUM(utb.total_tokens), 0)::bigint AS total_tokens,
  COUNT(*)::bigint AS call_count
FROM usage_events ue
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
WHERE ue.user_id = $1
  AND ue.created_at >= $2
  AND ue.created_at < $3
GROUP BY COALESCE(NULLIF(ue.model_routed, ''), 'unknown')
ORDER BY total_tokens DESC, model ASC
LIMIT $4`

	rows, err := s.db.QueryContext(ctx, query, userID, since.UTC(), until.UTC(), topN)
	if err != nil {
		return nil, fmt.Errorf("query user usage model top: %w", err)
	}
	defer rows.Close()

	out := []userUsageModelTop{}
	for rows.Next() {
		var item userUsageModelTop
		if err := rows.Scan(&item.Model, &item.Tokens, &item.CallCount); err != nil {
			return nil, fmt.Errorf("scan user usage model top: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user usage model top: %w", err)
	}
	return out, nil
}

func (s *postgresOverviewStore) loadUserUsageHourly(ctx context.Context, userID uuid.UUID, since time.Time, until time.Time) ([]int64, error) {
	const query = `
SELECT
  EXTRACT(HOUR FROM ue.created_at AT TIME ZONE 'UTC')::int AS hour_utc,
  COUNT(*)::bigint AS call_count
FROM usage_events ue
WHERE ue.user_id = $1
  AND ue.created_at >= $2
  AND ue.created_at < $3
GROUP BY EXTRACT(HOUR FROM ue.created_at AT TIME ZONE 'UTC')::int
ORDER BY hour_utc ASC`

	rows, err := s.db.QueryContext(ctx, query, userID, since.UTC(), until.UTC())
	if err != nil {
		return nil, fmt.Errorf("query user usage hourly distribution: %w", err)
	}
	defer rows.Close()

	hourly := make([]int64, 24)
	for rows.Next() {
		var hour int
		var count int64
		if err := rows.Scan(&hour, &count); err != nil {
			return nil, fmt.Errorf("scan user usage hourly distribution: %w", err)
		}
		if hour >= 0 && hour < len(hourly) {
			hourly[hour] = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user usage hourly distribution: %w", err)
	}
	return hourly, nil
}

func (s *postgresOverviewStore) loadUserUsageRecentCalls(ctx context.Context, userID uuid.UUID, since time.Time, until time.Time) ([]userUsageRecentCall, error) {
	const query = `
SELECT
  ue.created_at,
  COALESCE(NULLIF(ue.model_routed, ''), 'unknown') AS model,
  COALESCE(ue.status_code, 0)::int AS status_code,
  COALESCE(utb.total_tokens, 0)::bigint AS total_tokens,
  ue.streaming
FROM usage_events ue
LEFT JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
WHERE ue.user_id = $1
  AND ue.created_at >= $2
  AND ue.created_at < $3
ORDER BY ue.created_at DESC
LIMIT 50`

	rows, err := s.db.QueryContext(ctx, query, userID, since.UTC(), until.UTC())
	if err != nil {
		return nil, fmt.Errorf("query user usage recent calls: %w", err)
	}
	defer rows.Close()

	out := []userUsageRecentCall{}
	for rows.Next() {
		var createdAt time.Time
		var item userUsageRecentCall
		if err := rows.Scan(&createdAt, &item.Model, &item.StatusCode, &item.TotalTokens, &item.Streaming); err != nil {
			return nil, fmt.Errorf("scan user usage recent calls: %w", err)
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user usage recent calls: %w", err)
	}
	return out, nil
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
SELECT name, real_model, COALESCE(provider, 'ark') AS provider, status, COALESCE(description, '') as description
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
		if err := rows.Scan(&item.Name, &item.RealModel, &item.Provider, &item.Status, &item.Description); err != nil {
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

func (s *postgresOverviewStore) CreateCredential(ctx context.Context, params credentials.CreateParams) (credentials.PublicCredential, error) {
	if s == nil || s.envelope == nil {
		if s != nil && s.envelopeErr != nil {
			return credentials.PublicCredential{}, s.envelopeErr
		}
		return credentials.PublicCredential{}, omnicrypto.ErrMasterKeyMissing
	}
	return credentials.NewPostgresStore(s.db, s.envelope).Create(ctx, params)
}

func (s *postgresOverviewStore) DisableCredential(ctx context.Context, id string) (credentials.PublicCredential, error) {
	if s == nil || s.db == nil {
		return credentials.PublicCredential{}, credentials.ErrCredentialMissing
	}
	return credentials.NewPostgresStore(s.db, nil).Disable(ctx, id)
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
