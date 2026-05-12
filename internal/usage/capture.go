package usage

import "net/http"

type captureResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	tail        *tailBuffer
}

func newCaptureResponseWriter(w http.ResponseWriter, limit int64) *captureResponseWriter {
	return &captureResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
		tail:           newTailBuffer(limit),
	}
}

func (w *captureResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *captureResponseWriter) Write(p []byte) (int, error) {
	w.tail.Write(p)
	return w.ResponseWriter.Write(p)
}

func (w *captureResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *captureResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *captureResponseWriter) Status() int {
	return w.status
}

func (w *captureResponseWriter) Captured() []byte {
	return w.tail.Bytes()
}

type tailBuffer struct {
	limit int
	data  []byte
}

func newTailBuffer(limit int64) *tailBuffer {
	if limit <= 0 {
		limit = DefaultCaptureLimit
	}
	return &tailBuffer{limit: int(limit)}
}

func (b *tailBuffer) Write(p []byte) {
	if b.limit <= 0 || len(p) == 0 {
		return
	}
	if len(p) >= b.limit {
		b.data = append(b.data[:0], p[len(p)-b.limit:]...)
		return
	}
	overflow := len(b.data) + len(p) - b.limit
	if overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
	}
	b.data = append(b.data, p...)
}

func (b *tailBuffer) Bytes() []byte {
	out := make([]byte, len(b.data))
	copy(out, b.data)
	return out
}
