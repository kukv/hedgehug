package server

import (
	"context"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

// loopServer is a test server with a fake clock, a remote, and its loop
// running until the test ends.
func loopServer(t *testing.T) (*testServer, *fakeClock, string) {
	t.Helper()
	clk := newFakeClock(t0)
	ts := newTestServerWith(t, func(o *Options) { o.Clock, o.Now = clk, clk.Now })
	remote := ts.addRemote()
	ctx, cancel := context.WithCancel(context.Background())
	ts.srv.Start(ctx)
	t.Cleanup(func() {
		cancel()
		_ = ts.srv.Shutdown(context.Background())
	})
	return ts, clk, remote
}

func TestLoopSyncsAtStart(t *testing.T) {
	ts, _, _ := loopServer(t)
	waitFirstSync(t, ts)
}

func TestLoopIntervalVisibleAndHidden(t *testing.T) {
	ts, clk, _ := loopServer(t)
	waitFirstSync(t, ts)
	first := ts.state().LastAttempt

	// Hidden (no client): 2 minutes is not enough, 10 is.
	clk.Advance(2 * time.Minute)
	time.Sleep(100 * time.Millisecond)
	if got := ts.state().LastAttempt; !got.Equal(first) {
		t.Fatalf("hidden: synced after 2 minutes (%v)", got)
	}
	clk.Advance(8 * time.Minute)
	eventually(t, "the hidden fetch", func() bool { return ts.state().LastAttempt.After(first) })
	second := ts.state().LastAttempt

	// Visible: every 2 minutes.
	id := ts.srv.presence.Connect()
	defer ts.srv.presence.Disconnect(id)
	clk.Advance(2 * time.Minute)
	eventually(t, "the visible fetch", func() bool { return ts.state().LastAttempt.After(second) })
}

func TestLoopPushesAfterWrite(t *testing.T) {
	ts, clk, remote := loopServer(t)
	waitFirstSync(t, ts)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	if ts.state().PendingSince.IsZero() {
		t.Fatal("the write is not pending")
	}
	clk.Advance(29 * time.Second)
	time.Sleep(100 * time.Millisecond)
	if remoteHead(t, remote) == *out.Commit {
		t.Fatal("pushed before push_interval")
	}
	clk.Advance(time.Second)
	eventually(t, "the push", func() bool { return remoteHead(t, remote) == *out.Commit })
	// sync.json is written after the push.
	eventually(t, "the write to stop pending", func() bool { return ts.state().PendingSince.IsZero() })
}

func TestShutdownWaitsForPush(t *testing.T) {
	ts, clk, remote := loopServer(t)
	waitFirstSync(t, ts)
	_ = clk
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	if err := ts.srv.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := remoteHead(t, remote); got != *out.Commit {
		t.Errorf("remote at %s after shutdown, want %s", got, *out.Commit)
	}
}

func TestDrainGivesUp(t *testing.T) {
	ts, _, _ := loopServer(t)
	waitFirstSync(t, ts)
	block := make(chan struct{})
	defer close(block)
	ts.srv.loop.setSyncFn(func(string) { <-block })
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	start := time.Now()
	ts.srv.loop.Drain(200 * time.Millisecond)
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Drain took %v, want about 200ms", d)
	}
}
