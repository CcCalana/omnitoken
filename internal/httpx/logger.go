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

			attrs := []any{
				"request_id", RequestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.Status(),
				"duration_us", time.Since(start).Microseconds(),
			}
			if upstreamRequestID := UpstreamRequestIDFromContext(r.Context()); upstreamRequestID != "" {
				attrs = append(attrs, "upstream_request_id", upstreamRequestID)
			}

			logger.Info("http request", attrs...)
		})
	}
}
