package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// 止めるときは、処理中の要求が終わるのを待ってから serve が戻る。
func TestServeWaitsForInFlightRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "ok")
	})
	ctx, cancel := context.WithCancel(t.Context())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, ln, h) }()

	got := make(chan string, 1)
	go func() {
		res, err := http.Get("http://" + ln.Addr().String() + "/") //nolint:noctx // 止める ctx と別に、要求は最後まで通す
		if err != nil {
			got <- "error: " + err.Error()
			return
		}
		defer func() { _ = res.Body.Close() }()
		b, _ := io.ReadAll(res.Body)
		got <- string(b)
	}()

	<-started
	cancel()
	select {
	case err := <-served:
		t.Fatalf("serve returned while a request was in flight: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if body := <-got; body != "ok" {
		t.Errorf("in-flight request: %q, want ok", body)
	}
	if err := <-served; err != nil {
		t.Errorf("serve: %v", err)
	}
}
