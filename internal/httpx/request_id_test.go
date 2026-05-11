package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDReusesHeader(t *testing.T) {
	t.Parallel()

	var got string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "req-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got != "req-123" {
		t.Fatalf("request id = %q", got)
	}
	if rec.Header().Get(RequestIDHeader) != "req-123" {
		t.Fatalf("response header request id = %q", rec.Header().Get(RequestIDHeader))
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	t.Parallel()

	var got string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got == "" {
		t.Fatal("expected generated request id")
	}
	if rec.Header().Get(RequestIDHeader) != got {
		t.Fatalf("response header request id = %q, context = %q", rec.Header().Get(RequestIDHeader), got)
	}
}
