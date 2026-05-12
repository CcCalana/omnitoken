package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/proxy"
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

	server := &http.Server{
		Addr:              cfg.Gateway.Addr,
		Handler:           newMux(logger, auth.NewPostgresStore(db), newChatHandler(cfg, logger, db)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("gateway listening", "addr", cfg.Gateway.Addr)
	if err := httpx.Run(context.Background(), server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func newMux(logger *slog.Logger, store auth.VirtualKeyStore, chatHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.Handle("GET /v1/models", protectGatewayRoute(store, http.HandlerFunc(handleModels)))
	mux.Handle("POST /v1/chat/completions", protectGatewayRoute(store, chatHandler))

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
