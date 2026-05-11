package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusRecorderDefaultsToOK(t *testing.T) {
	t.Parallel()

	rec := NewStatusRecorder(httptest.NewRecorder())
	if rec.Status() != http.StatusOK {
		t.Fatalf("status = %d", rec.Status())
	}
}

func TestStatusRecorderTracksWriteHeader(t *testing.T) {
	t.Parallel()

	base := httptest.NewRecorder()
	rec := NewStatusRecorder(base)

	rec.WriteHeader(http.StatusTeapot)

	if rec.Status() != http.StatusTeapot {
		t.Fatalf("status = %d", rec.Status())
	}
	if base.Code != http.StatusTeapot {
		t.Fatalf("base status = %d", base.Code)
	}
}

func TestStatusRecorderFlushAndUnwrap(t *testing.T) {
	t.Parallel()

	base := httptest.NewRecorder()
	rec := NewStatusRecorder(base)

	rec.Flush()

	if !base.Flushed {
		t.Fatal("expected wrapped recorder to be flushed")
	}
	if rec.Unwrap() != base {
		t.Fatal("unexpected unwrapped response writer")
	}
}
