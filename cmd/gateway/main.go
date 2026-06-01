package main

import (
	"bytes"
	"context"
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

	_ "github.com/lib/pq"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/credentials"
	omnicrypto "github.com/omnitoken/omnitoken/internal/crypto"
	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/proxy"
	"github.com/omnitoken/omnitoken/internal/quota"
	"github.com/omnitoken/omnitoken/internal/router"
	"github.com/omnitoken/omnitoken/internal/usage"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Time    string `json:"time"`
}

type modelResponse struct {
	Object string      `json:"object"`
	Data   []modelItem `json:"data"`
}

type modelItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if cfg.DatabaseURL == "" {
		logger.Error("gateway requires OMNITOKEN_DATABASE_URL for Demo-Ready virtual key auth")
		os.Exit(1)
	}
	db, err := sql.Open("postgres", postgresURLWithApplicationName(cfg.DatabaseURL, "omnitoken-gateway"))
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close postgres", "error", err)
		}
	}()

	resolver := router.NewPostgresStore(db)
	modelCatalog, err := loadModelCatalog(context.Background(), db)
	if err != nil {
		logger.Error("load model catalog", "error", err)
		os.Exit(1)
	}
	credentialStore, selector := loadCredentialSelector(context.Background(), logger, db, cfg)
	if selector != nil {
		startCredentialPolling(context.Background(), logger, credentialStore, selector, credentialPollInterval(logger))
	}

	server := &http.Server{
		Addr:              cfg.Gateway.Addr,
		Handler:           newMux(logger, auth.NewPostgresStore(db), quota.NewChecker(quota.NewPostgresStore(db)), resolver, newChatHandler(cfg, logger, db, selector, modelCatalog)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("gateway listening", "addr", cfg.Gateway.Addr)
	if err := httpx.Run(context.Background(), server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
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

func newMux(logger *slog.Logger, store auth.VirtualKeyStore, budgetChecker quota.BudgetChecker, resolver router.Resolver, chatHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.Handle("GET /v1/models", protectGatewayRoute(store, http.HandlerFunc(handleModels)))
	mux.Handle("POST /v1/chat/completions", protectGatewayRoute(store, enforceMonthlyBudget(logger, budgetChecker)(resolveVirtualModel(resolver)(chatHandler))))
	mux.Handle("POST /v1/messages", protectGatewayRoute(store,
		enforceMonthlyBudget(logger, budgetChecker)(
			resolveVirtualModel(resolver)(
				proxy.NewAnthropicMessagesHandler(chatHandler, logger, proxy.AnthropicMessagesConfig{}),
			),
		),
	))

	return httpx.RequestID(httpx.RequestLogger(logger)(mux))
}

type credentialLoader interface {
	Load(context.Context) ([]credentials.Credential, error)
}

func loadCredentialSelector(ctx context.Context, logger *slog.Logger, db *sql.DB, cfg config.Config) (credentialLoader, *credentials.Selector) {
	masterKey, err := omnicrypto.LoadMasterKey(cfg.MasterKeyFile, cfg.MasterKey)
	if err != nil {
		if cfg.Ark.Enabled() {
			logger.Warn("upstream credential pool disabled; falling back to OMNITOKEN_ARK_API_KEY")
			return nil, nil
		}
		logger.Warn("upstream credential pool disabled", "err", err)
		return nil, nil
	}
	envelope, err := omnicrypto.NewEnvelope(masterKey)
	if err != nil {
		logger.Warn("upstream credential pool disabled", "err", err)
		return nil, nil
	}
	store := credentials.NewPostgresStore(db, envelope)
	items, err := store.Load(ctx)
	if err != nil {
		logger.Error("load upstream credentials", "err", err)
		return nil, nil
	}
	logger.Info("loaded upstream credentials", "count", len(items))
	return store, credentials.NewSelector(items)
}

func startCredentialPolling(ctx context.Context, logger *slog.Logger, store credentialLoader, selector *credentials.Selector, interval time.Duration) {
	if store == nil || selector == nil || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				items, err := store.Load(ctx)
				if err != nil {
					logger.Warn("credential pool reload failed", "err", err)
					continue
				}
				delta := selector.Replace(items)
				logger.Info("credential pool reloaded", "count", len(items), "delta", delta)
			}
		}
	}()
}

func credentialPollInterval(logger *slog.Logger) time.Duration {
	raw := strings.TrimSpace(os.Getenv("OMNITOKEN_CREDENTIAL_POLL_INTERVAL"))
	if raw == "" {
		return 30 * time.Second
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval < 0 {
		logger.Warn("invalid credential poll interval", "env", "OMNITOKEN_CREDENTIAL_POLL_INTERVAL", "value", raw, "default", "30s")
		return 30 * time.Second
	}
	return interval
}

func loadModelCatalog(ctx context.Context, db *sql.DB) (proxy.ModelCatalog, error) {
	const query = `
SELECT provider, canonical_model, provider_model
FROM model_catalog
WHERE status = 'active'
ORDER BY provider, canonical_model`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query model catalog: %w", err)
	}
	defer rows.Close()

	models := []proxy.ProviderModel{}
	for rows.Next() {
		var model proxy.ProviderModel
		if err := rows.Scan(&model.Provider, &model.CanonicalModel, &model.ProviderModel); err != nil {
			return nil, fmt.Errorf("scan model catalog: %w", err)
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model catalog: %w", err)
	}
	return proxy.NewStaticModelCatalog(models), nil
}

func newArkChatProxy(cfg config.Config, logger *slog.Logger, selector *credentials.Selector, modelCatalog proxy.ModelCatalog) http.Handler {
	return proxy.NewArkChatProxy(proxy.ArkChatConfig{
		BaseURL:            cfg.Ark.OpenAIBaseURL,
		APIKey:             cfg.Ark.APIKey,
		DefaultModel:       cfg.Ark.DefaultModel,
		DisableThinking:    cfg.Ark.DisableThinking,
		CredentialSelector: selector,
		ModelCatalog:       modelCatalog,
	}, logger, nil)
}

func newChatHandler(cfg config.Config, logger *slog.Logger, db *sql.DB, selector *credentials.Selector, modelCatalog proxy.ModelCatalog) http.Handler {
	return usage.Middleware(
		usage.NewRecorder(usage.NewPostgresStore(db), logger),
		usage.MiddlewareConfig{
			Provider:      "ark",
			ModelFallback: cfg.Ark.DefaultModel,
			Logger:        logger,
		},
	)(newArkChatProxy(cfg, logger, selector, modelCatalog))
}

func protectGatewayRoute(store auth.VirtualKeyStore, next http.Handler) http.Handler {
	return auth.RequireVirtualKey(store, writeGatewayUnauthorized)(next)
}

func writeGatewayUnauthorized(w http.ResponseWriter, r *http.Request) {
	if isAnthropicMessagesPath(r.URL.Path) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(anthropicErrorBytes("authentication_error", "invalid x-api-key"))
		return
	}
	httpx.WriteJSON(w, http.StatusUnauthorized, errorEnvelope{
		Error: errorDetail{
			Message: "unauthorized",
			Type:    "authentication_error",
			Code:    "invalid_api_key",
		},
	})
}

func isAnthropicMessagesPath(path string) bool {
	return strings.HasPrefix(path, "/v1/messages")
}

func enforceMonthlyBudget(logger *slog.Logger, checker quota.BudgetChecker) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		if checker == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, ok := auth.SubjectFromContext(r.Context())
			if !ok {
				logger.Error("quota subject missing", "request_id", httpx.RequestIDFromContext(r.Context()))
				writeGatewayQuotaCheckFailed(w)
				return
			}
			decision, err := checker.Check(r.Context(), subject, time.Now())
			if err != nil {
				logger.Error("quota check failed",
					"request_id", httpx.RequestIDFromContext(r.Context()),
					"organization_id", subject.OrgID.String(),
					"user_id", subject.UserID.String(),
					"err", err,
				)
				writeGatewayQuotaCheckFailed(w)
				return
			}
			if !decision.Allowed {
				var budget any
				if decision.BudgetCents.Valid {
					budget = decision.BudgetCents.Int64
				}
				logger.Warn("monthly budget exhausted",
					"request_id", httpx.RequestIDFromContext(r.Context()),
					"organization_id", subject.OrgID.String(),
					"user_id", subject.UserID.String(),
					"used_cents", decision.UsedBudgetCents,
					"budget_cents", budget,
				)
				writeGatewayQuotaExceeded(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func resolveVirtualModel(resolver router.Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if resolver == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope{
					Error: errorDetail{
						Message: "invalid request body",
						Type:    "invalid_request",
						Code:    "invalid_request",
					},
				})
				return
			}
			r.Body.Close()

			var payload map[string]any
			decoder := json.NewDecoder(bytes.NewReader(body))
			decoder.UseNumber()
			if err := decoder.Decode(&payload); err != nil {
				r.Body = io.NopCloser(bytes.NewReader(body))
				next.ServeHTTP(w, r)
				return
			}

			modelRequested, ok := payload["model"].(string)
			if !ok || modelRequested == "" {
				r.Body = io.NopCloser(bytes.NewReader(body))
				next.ServeHTTP(w, r)
				return
			}

			res, err := resolver.Resolve(r.Context(), modelRequested)
			if err != nil {
				if errors.Is(err, router.ErrVirtualModelDisabled) {
					httpx.WriteJSON(w, http.StatusBadRequest, errorEnvelope{
						Error: errorDetail{
							Message: "virtual model is disabled",
							Type:    "invalid_request",
							Code:    "virtual_model_disabled",
						},
					})
					return
				}
				httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope{
					Error: errorDetail{
						Message: "failed to resolve virtual model",
						Type:    "server_error",
						Code:    "internal_error",
					},
				})
				return
			}

			if res.IsVirtual {
				payload["model"] = res.RealModel
				newBody, err := json.Marshal(payload)
				if err == nil {
					body = newBody
					r.ContentLength = int64(len(body))
					r.Header.Set("Content-Length", strconv.Itoa(len(body)))
				}
				ctx := httpx.WithVirtualModel(r.Context(), modelRequested)
				ctx = httpx.WithModelRouted(ctx, res.RealModel)
				ctx = httpx.WithProviderRouted(ctx, res.Provider)
				r = r.WithContext(ctx)
			} else {
				r = r.WithContext(httpx.WithModelRouted(r.Context(), modelRequested))
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "gateway",
		Time:    time.Now().UTC().Format(time.RFC3339),
	})
}

func handleModels(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, modelResponse{
		Object: "list",
		Data: []modelItem{
			{ID: "gpt-4o", Object: "model", Created: 1715558400, OwnedBy: "openai"},
			{ID: "gpt-4o-mini", Object: "model", Created: 1721347200, OwnedBy: "openai"},
			{ID: "claude-3-5-sonnet", Object: "model", Created: 1718841600, OwnedBy: "anthropic"},
			{ID: "gemini-1.5-pro", Object: "model", Created: 1714435200, OwnedBy: "google"},
			{ID: "ark-code-latest", Object: "model", Created: 1746921600, OwnedBy: "ark"},
		},
	})
}

func anthropicErrorBytes(errorType string, message string) []byte {
	body, _ := json.Marshal(map[string]any{
		"type": "error",
		"error": map[string]string{
			"type":    errorType,
			"message": message,
		},
	})
	return body
}

func writeGatewayQuotaExceeded(w http.ResponseWriter) {
	httpx.WriteJSON(w, http.StatusPaymentRequired, errorEnvelope{
		Error: errorDetail{
			Message: "monthly budget exhausted",
			Type:    "quota_exceeded",
			Code:    "monthly_budget_exhausted",
		},
	})
}

func writeGatewayQuotaCheckFailed(w http.ResponseWriter) {
	httpx.WriteJSON(w, http.StatusInternalServerError, errorEnvelope{
		Error: errorDetail{
			Message: "quota check failed",
			Type:    "server_error",
			Code:    "quota_check_failed",
		},
	})
}
