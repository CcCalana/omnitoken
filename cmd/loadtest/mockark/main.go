package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type chatRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Content string `json:"content"`
	} `json:"messages"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Message      any    `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage usageResponse `json:"usage"`
}

type usageResponse struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

var requestSeq atomic.Int64

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/coding/v3/chat/completions", handleChat(logger))
	mux.HandleFunc("POST /chat/completions", handleChat(logger))

	server := &http.Server{
		Addr:              ":8090",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("mock ark listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("mock ark stopped", "error", err)
		os.Exit(1)
	}
}

func handleChat(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing bearer token", "invalid_request")
			return
		}
		defer r.Body.Close()
		var body chatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body", "invalid_request")
			return
		}
		if body.Stream {
			writeError(w, http.StatusBadRequest, "stream is not supported by mockark", "invalid_request")
			return
		}
		promptTokens := 8 + len(body.Messages)
		completionTokens := 2
		response := chatResponse{
			ID:      "chatcmpl-mock-" + time.Now().UTC().Format("150405.000000000"),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   firstNonEmpty(body.Model, "mock-ark-model"),
			Usage: usageResponse{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      promptTokens + completionTokens,
			},
		}
		response.Choices = append(response.Choices, struct {
			Index        int    `json:"index"`
			Message      any    `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			Index:        0,
			Message:      map[string]string{"role": "assistant", "content": "pong"},
			FinishReason: "stop",
		})

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Warn("write response", "error", err)
			return
		}
		requestSeq.Add(1)
	}
}

func writeError(w http.ResponseWriter, status int, message string, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    "mock_error",
			"code":    code,
		},
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
