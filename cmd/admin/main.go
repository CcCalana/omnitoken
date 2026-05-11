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

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	server := &http.Server{
		Addr:              cfg.Admin.Addr,
		Handler:           newMux(logger, cfg.Admin),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("admin listening", "addr", cfg.Admin.Addr)
	if err := httpx.Run(context.Background(), server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("admin stopped", "error", err)
		os.Exit(1)
	}
}

func newMux(logger *slog.Logger, cfg config.AdminConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /api/admin/overview", handleOverview)

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
