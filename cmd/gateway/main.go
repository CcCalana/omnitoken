package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
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
	db, err := sql.Open("postgres", cfg.DatabaseURL)
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

	server := &http.Server{
		Addr:              cfg.Gateway.Addr,
		Handler:           newMux(logger, auth.NewPostgresStore(db), quota.NewChecker(quota.NewPostgresStore(db)), resolver, newChatHandler(cfg, logger, db)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("gateway listening", "addr", cfg.Gateway.Addr)
	if err := httpx.Run(context.Background(), server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func newMux(logger *slog.Logger, store auth.VirtualKeyStore, budgetChecker quota.BudgetChecker, resolver router.Resolver, chatHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.Handle("GET /v1/models", protectGatewayRoute(store, http.HandlerFunc(handleModels)))
	mux.Handle("POST /v1/chat/completions", protectGatewayRoute(store, enforceMonthlyBudget(logger, budgetChecker)(resolveVirtualModel(resolver)(chatHandler))))

	return httpx.RequestID(httpx.RequestLogger(logger)(mux))
}

func newArkChatProxy(cfg config.Config, logger *slog.Logger) http.Handler {
	return proxy.NewArkChatProxy(proxy.ArkChatConfig{
		BaseURL:         cfg.Ark.OpenAIBaseURL,
		APIKey:          cfg.Ark.APIKey,
		DefaultModel:    cfg.Ark.DefaultModel,
		DisableThinking: cfg.Ark.DisableThinking,
	}, logger, nil)
}

func newChatHandler(cfg config.Config, logger *slog.Logger, db *sql.DB) http.Handler {
	return usage.Middleware(
		usage.NewRecorder(usage.NewPostgresStore(db), logger),
		usage.MiddlewareConfig{
			Provider:      "ark",
			ModelFallback: cfg.Ark.DefaultModel,
			Logger:        logger,
		},
	)(newArkChatProxy(cfg, logger))
}

func protectGatewayRoute(store auth.VirtualKeyStore, next http.Handler) http.Handler {
	return auth.RequireVirtualKey(store, writeGatewayUnauthorized)(next)
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
				r = r.WithContext(ctx)
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

func writeGatewayUnauthorized(w http.ResponseWriter) {
	httpx.WriteJSON(w, http.StatusUnauthorized, errorEnvelope{
		Error: errorDetail{
			Message: "unauthorized",
			Type:    "authentication_error",
			Code:    "invalid_api_key",
		},
	})
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
