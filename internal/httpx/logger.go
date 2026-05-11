package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := NewStatusRecorder(w)

			next.ServeHTTP(rec, r)

			logger.Info(
				"http request",
				"request_id", RequestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.Status(),
				"duration_us", time.Since(start).Microseconds(),
			)
		})
	}
}
