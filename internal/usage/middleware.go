package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/httpx"
)

type MiddlewareConfig struct {
	Provider      string
	ModelFallback string
	CaptureLimit  int64
	RecordTimeout time.Duration
	Logger        *slog.Logger
	Now           func() time.Time
}

func Middleware(recorder Recorder, cfg MiddlewareConfig) func(http.Handler) http.Handler {
	cfg = cfg.withDefaults()
	return func(next http.Handler) http.Handler {
		if recorder == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := cfg.Now()
			modelRequested, streaming := snapshotRequestMetadata(r)
			capture := newCaptureResponseWriter(w, cfg.CaptureLimit)

			next.ServeHTTP(capture, r)

			if capture.Status() < http.StatusOK || capture.Status() >= http.StatusMultipleChoices {
				return
			}
			subject, ok := auth.SubjectFromContext(r.Context())
			if !ok {
				cfg.Logger.Warn("usage subject missing", "request_id", httpx.RequestIDFromContext(r.Context()))
				return
			}

			input := RecordInput{
				RequestID:      httpx.RequestIDFromContext(r.Context()),
				Subject:        subject,
				ModelRequested: modelRequested,
				ModelFallback:  cfg.ModelFallback,
				Provider:       cfg.Provider,
				StatusCode:     capture.Status(),
				LatencyMS:      int(cfg.Now().Sub(start).Milliseconds()),
				Streaming:      streaming,
				Captured:       capture.Captured(),
			}

			go func(input RecordInput) {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.RecordTimeout)
				defer cancel()
				if err := recorder.Record(ctx, input); err != nil {
					cfg.Logger.Error("usage record failed", "request_id", input.RequestID, "err", err)
				}
			}(input)
		})
	}
}

func (cfg MiddlewareConfig) withDefaults() MiddlewareConfig {
	if cfg.Provider == "" {
		cfg.Provider = "ark"
	}
	if cfg.CaptureLimit <= 0 {
		cfg.CaptureLimit = DefaultCaptureLimit
	}
	if cfg.RecordTimeout <= 0 {
		cfg.RecordTimeout = DefaultRecordTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return cfg
}

func snapshotRequestMetadata(r *http.Request) (string, bool) {
	if r.Body == nil {
		return "", false
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(nil))
		return "", false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", false
	}
	return payload.Model, payload.Stream
}
