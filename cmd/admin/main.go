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

type postgresVirtualKeyCreator struct {
	db     *sql.DB
	random io.Reader
	now    func() time.Time
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	creator, closeCreator := newVirtualKeyCreator(logger, cfg)
	defer closeCreator()

	server := &http.Server{
		Addr:              cfg.Admin.Addr,
		Handler:           newMux(logger, cfg.Admin, creator),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("admin listening", "addr", cfg.Admin.Addr)
	if err := httpx.Run(context.Background(), server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("admin stopped", "error", err)
		os.Exit(1)
	}
}

func newVirtualKeyCreator(logger *slog.Logger, cfg config.Config) (virtualKeyCreator, func()) {
	if cfg.Admin.BootstrapToken == "" {
		logger.Info("admin dev virtual key endpoint disabled", "reason", "bootstrap_token_empty", "port", cfg.Admin.Addr)
		return nil, func() {}
	}
	if cfg.DatabaseURL == "" {
		logger.Error("admin dev virtual key endpoint requires OMNITOKEN_DATABASE_URL")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	return &postgresVirtualKeyCreator{db: db, random: nil, now: time.Now}, func() {
		if err := db.Close(); err != nil {
			logger.Error("close postgres", "error", err)
		}
	}
}

func newMux(logger *slog.Logger, cfg config.AdminConfig, creator virtualKeyCreator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /api/admin/overview", handleOverview)
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

func handleOverview(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, overviewResponse{
		Period:            "2026-05",
		TotalTokens:       42_500_000,
		EstimatedCostUSD:  1240.50,
		ActiveUsers:       128,
		QuotaWarningUsers: 12,
		Trend: []dailyTokenUsage{
			{Date: "2026-05-01", Tokens: 1_200_000, Cost: 35.10},
			{Date: "2026-05-02", Tokens: 1_500_000, Cost: 44.80},
			{Date: "2026-05-03", Tokens: 1_100_000, Cost: 31.90},
		},
		ModelUsage: []modelUsage{
			{Model: "gpt-4o", Tokens: 19_125_000, Cost: 558.22, Share: 0.45},
			{Model: "claude-3-5-sonnet", Tokens: 10_625_000, Cost: 341.13, Share: 0.25},
			{Model: "gemini-1.5-pro", Tokens: 6_375_000, Cost: 188.40, Share: 0.15},
			{Model: "gpt-4o-mini", Tokens: 6_375_000, Cost: 152.75, Share: 0.15},
		},
	})
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
