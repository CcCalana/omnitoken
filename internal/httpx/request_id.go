package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const (
	RequestIDHeader         = "X-Request-Id"
	UpstreamRequestIDHeader = "X-Upstream-Request-Id"
)

type requestIDContextKey struct{}
type upstreamRequestIDContextKey struct{}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		upstreamRequestID := strings.TrimSpace(r.Header.Get(RequestIDHeader))

		w.Header().Set(RequestIDHeader, requestID)
		if upstreamRequestID != "" {
			w.Header().Set(UpstreamRequestIDHeader, upstreamRequestID)
		}

		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		ctx = context.WithValue(ctx, upstreamRequestIDContextKey{}, upstreamRequestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func UpstreamRequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(upstreamRequestIDContextKey{}).(string)
	return requestID
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
}
