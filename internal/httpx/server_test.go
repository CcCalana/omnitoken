package httpx

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunGracefullyShutsDown(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-release
			_, _ = w.Write([]byte("done"))
		}),
		ReadHeaderTimeout: time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		errc <- Run(ctx, srv)
	}()

	respCh := make(chan *http.Response, 1)
	errReqCh := make(chan error, 1)
	go func() {
		var lastErr error
		for i := 0; i < 100; i++ {
			resp, err := http.Get("http://" + addr)
			if err == nil {
				respCh <- resp
				return
			}
			lastErr = err
			time.Sleep(10 * time.Millisecond)
		}
		errReqCh <- lastErr
	}()

	select {
	case <-entered:
	case err := <-errReqCh:
		t.Fatalf("request failed: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not reached")
	}

	cancel()
	close(release)

	var resp *http.Response
	select {
	case resp = <-respCh:
	case err := <-errReqCh:
		t.Fatalf("request failed after shutdown: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("request did not complete")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "done" {
		t.Fatalf("body = %q", body)
	}

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}
}
