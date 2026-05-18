package audit

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/omnitoken/omnitoken/internal/httpx"
)

const DefaultRecordTimeout = 2 * time.Second

type ActorResolver func(*http.Request) Actor

type MiddlewareConfig struct {
	ActorResolver ActorResolver
	RecordTimeout time.Duration
	Logger        *slog.Logger
	Now           func() time.Time
}

func BootstrapActorResolver(*http.Request) Actor {
	return Actor{ID: ActorTypeBootstrap, Type: ActorTypeBootstrap}
}

func Middleware(recorder Recorder, cfg MiddlewareConfig) func(http.Handler) http.Handler {
	cfg = cfg.withDefaults()
	return func(next http.Handler) http.Handler {
		if recorder == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isWriteMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			ctx, scope := WithScope(r.Context())
			r = r.WithContext(ctx)
			status := httpx.NewStatusRecorder(w)
			panicked := false
			defer func() {
				if recovered := recover(); recovered != nil {
					panicked = true
					SetAction(r.Context(), ActionPanicRecovered)
					recordFromScope(context.Background(), recorder, cfg, r, scope, http.StatusInternalServerError)
					panic(recovered)
				}
				if !panicked {
					recordFromScope(context.Background(), recorder, cfg, r, scope, status.Status())
				}
			}()

			next.ServeHTTP(status, r)
		})
	}
}

func (cfg MiddlewareConfig) withDefaults() MiddlewareConfig {
	if cfg.ActorResolver == nil {
		cfg.ActorResolver = BootstrapActorResolver
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

func recordFromScope(ctx context.Context, recorder Recorder, cfg MiddlewareConfig, r *http.Request, scope *Scope, statusCode int) {
	action, resourceType, resourceID, before, after := scope.snapshot()
	entry := Entry{
		Actor:        cfg.ActorResolver(r),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Before:       before,
		After:        after,
		IP:           remoteIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		RequestID:    httpx.RequestIDFromContext(r.Context()),
		StatusCode:   statusCode,
		CreatedAt:    cfg.Now().UTC(),
	}

	go func(entry Entry) {
		recordCtx, cancel := context.WithTimeout(ctx, cfg.RecordTimeout)
		defer cancel()
		if err := recorder.Record(recordCtx, entry); err != nil {
			AuditRecordFailuresTotal.Add(1)
			cfg.Logger.Warn("audit record failed",
				"request_id", entry.RequestID,
				"action", defaultString(entry.Action, ActionUnknownAdminWrite),
				"resource_type", defaultString(entry.ResourceType, "admin_write"),
				"err", err,
			)
		}
	}(entry)
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}
	// Deployment behind LB: switch to trusted forwarded headers in a Phase 3
	// deployment runbook instead of trusting client-supplied headers here.
	return net.ParseIP(host)
}
