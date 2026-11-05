package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// A write to a store with no remote has nothing to push. The loop reads
// GitHub for it every interval, not push_interval after each write, and Drain
// does not wait for it.
func TestLoopDoesNotSyncAStoreWithoutRemoteAfterAWrite(t *testing.T) {
	t.Parallel()
	clk := newFakeClock(t0)
	ts := newTestServerWith(t, func(o *Options) { o.Clock, o.Now = clk, clk.Now })
	ts.uc.AddTask("a", func(x *model.Task) { x.Repos = []string{"github.com/org-a/web"} })
	var calls atomic.Int32
	ts.srv.loop.setSyncFn(func(name string) {
		calls.Add(1)
		ts.srv.loop.syncStore(name)
	})
	ctx, cancel := context.WithCancel(context.Background())
	ts.srv.Start(ctx)
	t.Cleanup(func() {
		cancel()
		_ = ts.srv.Shutdown(context.Background())
	})
	eventually(t, "the sync at start", func() bool { return calls.Load() == 1 })

	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	// The next read is an interval after the last one (hidden: ten minutes),
	// not push_interval after the write.
	settle(t, ts, clk, t0.Add(10*time.Minute))
	clk.Advance(5 * time.Minute)
	ts.srv.loop.Wake()
	eventually(t, "the loop to park", ts.srv.loop.parked)
	if n := calls.Load(); n != 1 {
		t.Fatalf("%d syncs after a write", n)
	}
	start := time.Now()
	ts.srv.loop.Drain(5 * time.Second)
	if d := time.Since(start); d > time.Second || calls.Load() != 1 {
		t.Errorf("Drain took %v and synced %d times", d, calls.Load()-1)
	}
	clk.Advance(5 * time.Minute)
	eventually(t, "the next read", func() bool { return calls.Load() == 2 })
}
