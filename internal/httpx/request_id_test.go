package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDGeneratesInternalIDAndPreservesClientID(t *testing.T) {
	t.Parallel()

	var got string
	var upstreamGot string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		upstreamGot = UpstreamRequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "req-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got == "" {
		t.Fatal("expected generated internal request id")
	}
	if got == "req-123" {
		t.Fatal("client request id should not overwrite internal request id")
	}
	if upstreamGot != "req-123" {
		t.Fatalf("upstream request id = %q", upstreamGot)
	}
	if rec.Header().Get(RequestIDHeader) != got {
		t.Fatalf("response header request id = %q", rec.Header().Get(RequestIDHeader))
	}
	if rec.Header().Get(UpstreamRequestIDHeader) != "req-123" {
		t.Fatalf("response upstream header = %q", rec.Header().Get(UpstreamRequestIDHeader))
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	t.Parallel()

	var got string
	var upstreamGot string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		upstreamGot = UpstreamRequestIDFromContext(r.Context())
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
	if upstreamGot != "" {
		t.Fatalf("unexpected upstream request id = %q", upstreamGot)
	}
	if rec.Header().Get(UpstreamRequestIDHeader) != "" {
		t.Fatalf("unexpected upstream response header = %q", rec.Header().Get(UpstreamRequestIDHeader))
	}
}
