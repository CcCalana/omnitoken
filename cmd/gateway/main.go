package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/omnitoken/omnitoken/internal/config"
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

func main() {
	addr := config.Env("OMNITOKEN_GATEWAY_ADDR", ":8080")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	server := &http.Server{
		Addr:              addr,
		Handler:           newMux(logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("gateway listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func newMux(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /v1/models", handleModels)

	return requestLogger(logger, mux)
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "gateway",
		Time:    time.Now().UTC().Format(time.RFC3339),
	})
}

func handleModels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, modelResponse{
		Object: "list",
		Data: []modelItem{
			{ID: "gpt-4o", Object: "model", Created: 1715558400, OwnedBy: "openai"},
			{ID: "gpt-4o-mini", Object: "model", Created: 1721347200, OwnedBy: "openai"},
			{ID: "claude-3-5-sonnet", Object: "model", Created: 1718841600, OwnedBy: "anthropic"},
			{ID: "gemini-1.5-pro", Object: "model", Created: 1714435200, OwnedBy: "google"},
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("write json response", "error", err)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		logger.Info(
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
