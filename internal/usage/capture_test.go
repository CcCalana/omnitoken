package usage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCaptureResponseWriterUnwrap(t *testing.T) {
	base := httptest.NewRecorder()
	capture := newCaptureResponseWriter(base, 16)
	if capture.Unwrap() != base {
		t.Fatal("unwrap did not return base writer")
	}
}

func TestTailBufferKeepsLastBytes(t *testing.T) {
	buffer := newTailBuffer(5)
	buffer.Write([]byte("hel"))
	buffer.Write([]byte("loworld"))
	if got := string(buffer.Bytes()); got != "world" {
		t.Fatalf("tail = %q", got)
	}
}

func TestTailBufferDefaultLimit(t *testing.T) {
	buffer := newTailBuffer(0)
	if buffer.limit != int(DefaultCaptureLimit) {
		t.Fatalf("limit = %d", buffer.limit)
	}
	buffer.Write(nil)
	if got := len(buffer.Bytes()); got != 0 {
		t.Fatalf("empty write changed buffer: %d", got)
	}
}

func TestCaptureResponseWriterDefaultsStatusOK(t *testing.T) {
	capture := newCaptureResponseWriter(httptest.NewRecorder(), 16)
	if capture.Status() != http.StatusOK {
		t.Fatalf("status = %d", capture.Status())
	}
}
