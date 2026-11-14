package events_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/events"
	"github.com/kukv/hedgehug/internal/app/port"
)

// 06「画面への反映」: つないでいる画面に、出来事を Server-Sent Events で送る。
func TestEventsStreamsPublishedEvents(t *testing.T) {
	hub := events.NewHub()
	mux := http.NewServeMux()
	events.Register(humago.New(mux, huma.DefaultConfig("t", "0")), hub)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}
	deadline := time.Now().Add(2 * time.Second)
	for hub.Listeners() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no listener")
		}
		time.Sleep(5 * time.Millisecond)
	}
	hub.Publish(port.Event{Kind: port.EventStore, Store: "S1"})
	hub.Publish(port.Event{Kind: port.EventSync, Store: "S2"})
	hub.Publish(port.Event{Kind: port.EventGitHub, Store: "S3"})
	hub.Publish(port.Event{Kind: port.EventInbox, Store: "S4"})
	var lines []string
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() && len(lines) < 8 {
		if l := sc.Text(); l != "" {
			lines = append(lines, l)
		}
	}
	want := []string{
		"event: store", `data: {"store":"S1"}`, "event: sync", `data: {"store":"S2"}`, "event: github", `data: {"store":"S3"}`,
		"event: inbox", `data: {"store":"S4"}`,
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("stream =\n%s", strings.Join(lines, "\n"))
	}
}

// 画面がつないでいなくても、Publish は待たない。切れた画面は数えない。
func TestPublishWithoutListeners(t *testing.T) {
	hub := events.NewHub()
	hub.Publish(port.Event{Kind: port.EventStore, Store: "S1"})
	if hub.Listeners() != 0 {
		t.Error("listeners")
	}
}

// waitListeners は、つないでいる画面が want になるまで待つ。
func waitListeners(t *testing.T, hub *events.Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for hub.Listeners() != want {
		if time.Now().After(deadline) {
			t.Fatalf("listeners = %d, want %d", hub.Listeners(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// drain は、url の流れを読み切るまでの結果を返す。ctx が切れたら、読みは止まる。
func drain(ctx context.Context, url string) <-chan error {
	ended := make(chan error, 1)
	go func() {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			ended <- err
			return
		}
		_, err = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		ended <- err
	}()
	return ended
}

// 決定 C2-27: Close で、つないでいる流れを終える(サーバーをすぐ止められるように)。Close のあとにつないだ
// 流れも、すぐ終わる。壊れたときはハングせずに失敗する。
func TestCloseEndsStreams(t *testing.T) {
	hub := events.NewHub()
	mux := http.NewServeMux()
	events.Register(humago.New(mux, huma.DefaultConfig("t", "0")), hub)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // srv.Close より先に、読みを止める
	first := drain(ctx, srv.URL+"/api/events")
	waitListeners(t, hub, 1)
	hub.Close()
	for i, ended := range []<-chan error{first, nil} {
		if i == 1 {
			ended = drain(ctx, srv.URL+"/api/events")
		}
		select {
		case err := <-ended:
			if err != nil {
				t.Errorf("stream %d ended with %v", i, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("stream %d did not end", i)
		}
	}
}

// 画面が切れたら、数えるのをやめる。
func TestDisconnectDropsListener(t *testing.T) {
	hub := events.NewHub()
	mux := http.NewServeMux()
	events.Register(humago.New(mux, huma.DefaultConfig("t", "0")), hub)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	_ = drain(ctx, srv.URL+"/api/events")
	waitListeners(t, hub, 1)
	cancel()
	waitListeners(t, hub, 0)
}
