package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/ops"
	hhsync "github.com/kukv/hedgehug/internal/sync"
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
	settle(t, ts, clk, t0.Add(10*time.Minute))
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
	settle(t, ts, clk, t0.Add(12*time.Minute))
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
	settle(t, ts, clk, t0.Add(30*time.Second))
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

// settle waits until the loop has decided on everything that happened so
// far and waits on the clock for at, so that the test may advance the clock.
func settle(t *testing.T, ts *testServer, clk *fakeClock, at time.Time) {
	t.Helper()
	eventually(t, "the loop to park", ts.srv.loop.parked)
	clk.waitWaiter(t, at)
}

func TestDrainSyncsAgainAfterARunningSync(t *testing.T) {
	ts, _, remote := loopServer(t)
	waitFirstSync(t, ts)
	var mu sync.Mutex
	calls := 0
	release := make(chan struct{})
	ts.srv.loop.setSyncFn(func(name string) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			<-release // began before the write, so it pushes nothing
			return
		}
		ts.srv.loop.syncStore(name)
	})
	ts.srv.loop.start("me")
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	go func() { time.Sleep(100 * time.Millisecond); close(release) }()
	ts.srv.loop.Drain(10 * time.Second)
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 || remoteHead(t, remote) != *out.Commit {
		t.Errorf("calls = %d, remote at %s, want 2 and %s", calls, remoteHead(t, remote), *out.Commit)
	}
}

func TestLoopDoesNotRetryWithinPushInterval(t *testing.T) {
	ts, clk, _ := loopServer(t)
	waitFirstSync(t, ts)
	var calls atomic.Int32
	ts.srv.loop.setSyncFn(func(string) { calls.Add(1) }) // fails: the write stays pending
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	settle(t, ts, clk, t0.Add(30*time.Second))
	clk.Advance(30 * time.Second)
	eventually(t, "the first try", func() bool { return calls.Load() == 1 })
	settle(t, ts, clk, t0.Add(60*time.Second))
	clk.Advance(29 * time.Second)
	time.Sleep(100 * time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Fatalf("%d syncs 29 seconds after a failed one", n)
	}
	clk.Advance(time.Second)
	eventually(t, "the second try", func() bool { return calls.Load() == 2 })
}

func TestLoopDoesNotOverlapSyncsOfAStore(t *testing.T) {
	ts, clk, _ := loopServer(t)
	waitFirstSync(t, ts)
	var calls atomic.Int32
	block := make(chan struct{})
	ts.srv.loop.setSyncFn(func(string) { calls.Add(1); <-block })
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	settle(t, ts, clk, t0.Add(30*time.Second))
	clk.Advance(30 * time.Second)
	eventually(t, "the sync to start", func() bool { return calls.Load() == 1 })
	clk.Advance(time.Hour)
	ts.srv.loop.Wake()
	time.Sleep(200 * time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Errorf("%d syncs of one store at once", n)
	}
	close(block)
}

func TestLoopFollowsAddedStores(t *testing.T) {
	ts, _, _ := loopServer(t)
	waitFirstSync(t, ts)
	remote := filepath.Join(t.TempDir(), "two.git")
	if err := gitrepo.InitBare(remote); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.session().StoreInit(ops.StoreInitInput{Name: "two", Prefix: "TW", Workspace: "two", Remotes: []string{remote}}); err != nil {
		t.Fatal(err)
	}
	ts.srv.loop.Wake()
	eventually(t, "the sync of the new store", func() bool {
		ss := ts.session()
		ss.StoreName = "two"
		s, err := ss.OpenStore()
		if err != nil {
			return false
		}
		st, err := hhsync.ReadState(s)
		return err == nil && !st.LastSuccess.IsZero()
	})
}

func TestLoopLogsBrokenConfigOnce(t *testing.T) {
	ts := newTestServer(t)
	cfg := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	good, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	broken := []byte("stores: [\n")
	if err := os.WriteFile(cfg, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	l := ts.srv.loop
	lines := func(ts *testServer) int { return strings.Count(ts.log.String(), "sync: ") }

	// From the loop (step) repeatedly, then from Synced: one line.
	l.step()
	l.step()
	l.Synced("me")
	if n := lines(ts); n != 1 {
		t.Fatalf("after step and Synced: %d lines\n%s", n, ts.log.String())
	}

	// Synced alone, before anything was logged, logs once.
	ts2 := newTestServer(t)
	if err := os.WriteFile(filepath.Join(ts2.env["HEDGEHUG_HOME"], "config.yaml"), broken, 0o600); err != nil {
		t.Fatal(err)
	}
	ts2.srv.loop.Synced("me")
	ts2.srv.loop.Synced("me")
	if n := lines(ts2); n != 1 {
		t.Fatalf("Synced alone: %d lines\n%s", n, ts2.log.String())
	}

	// Once it reads again the failure is forgotten, so a new one is logged.
	if err := os.WriteFile(cfg, good, 0o600); err != nil {
		t.Fatal(err)
	}
	l.step()
	if err := os.WriteFile(cfg, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	l.step()
	if n := lines(ts); n != 2 {
		t.Fatalf("after recovery and a new failure: %d lines\n%s", n, ts.log.String())
	}
}

func TestLoopLogsNoTargetsOnce(t *testing.T) {
	ts := newTestServer(t)
	cfg := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	good, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("stores: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := ts.srv.loop
	lines := func() int { return strings.Count(ts.log.String(), "sync: ") }

	// config.yaml reads but names no store: the same failure every minute is one line.
	l.step()
	l.step()
	l.step()
	if n := lines(); n != 1 {
		t.Fatalf("after three steps: %d lines\n%s", n, ts.log.String())
	}

	// Once it works again the failure is forgotten, so a new one is logged.
	if err := os.WriteFile(cfg, good, 0o600); err != nil {
		t.Fatal(err)
	}
	l.step()
	if err := os.WriteFile(cfg, []byte("stores: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l.step()
	if n := lines(); n != 2 {
		t.Fatalf("after recovery and a new failure: %d lines\n%s", n, ts.log.String())
	}
}
