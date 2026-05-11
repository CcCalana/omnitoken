package httpx

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggerIncludesRequestIDAndDuration(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := RequestID(RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(RequestIDHeader, "req-log")
	req.Header.Set("Authorization", "Bearer SECRET_value")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logLine := buf.String()
	if !strings.Contains(logLine, `"request_id":"`) {
		t.Fatalf("missing request_id in log: %s", logLine)
	}
	if strings.Contains(logLine, `"request_id":"req-log"`) {
		t.Fatalf("client request id overwrote internal request id: %s", logLine)
	}
	if !strings.Contains(logLine, `"upstream_request_id":"req-log"`) {
		t.Fatalf("missing upstream_request_id in log: %s", logLine)
	}
	if !strings.Contains(logLine, `"duration_us":`) {
		t.Fatalf("missing duration_us in log: %s", logLine)
	}
	if strings.Contains(logLine, "SECRET_value") || strings.Contains(logLine, "Authorization") {
		t.Fatalf("log leaked authorization data: %s", logLine)
	}
}
