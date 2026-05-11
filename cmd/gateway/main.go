package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/omnitoken/omnitoken/internal/config"
	"github.com/omnitoken/omnitoken/internal/httpx"
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

	server := &http.Server{
		Addr:              cfg.Gateway.Addr,
		Handler:           newMux(logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("gateway listening", "addr", cfg.Gateway.Addr)
	if err := httpx.Run(context.Background(), server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func newMux(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /v1/models", handleModels)
	mux.HandleFunc("POST /v1/chat/completions", handleChatCompletions)

	return httpx.RequestID(httpx.RequestLogger(logger)(mux))
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

func handleChatCompletions(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusBadGateway, errorEnvelope{
		Error: errorDetail{
			Message: "chat completions proxy is not implemented yet",
			Type:    "gateway_error",
			Code:    "upstream_not_configured",
		},
	})
}
