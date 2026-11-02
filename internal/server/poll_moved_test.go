package server

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/ops"
)

// A task that became a forward is no longer in the store's list: the poller
// tells so as removed, not as changed (a refetch would get task.moved).
func TestPollMovedTaskIsRemoved(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	ss := ts.session()
	ss.StoreName = "me"
	moved, err := ss.Add(ops.AddInput{Title: "moves"})
	if err != nil {
		t.Fatal(err)
	}
	ch, cancel := ts.srv.hub.Subscribe()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	ts.srv.poller.interval = 10 * time.Millisecond
	go func() {
		defer close(done)
		ts.srv.poller.Run(ctx)
	}()
	t.Cleanup(func() { stop(); cancel(); <-done })
	eventually(t, "the first look of the poller", func() bool {
		ts.srv.poller.mu.Lock()
		defer ts.srv.poller.mu.Unlock()
		return len(ts.srv.poller.stores) > 1
	})
	ts.srv.poller.settle()

	if _, err := ts.session().MoveStore(ops.MoveStoreInput{Arg: moved.ULID.String(), To: "team", Yes: true}); err != nil {
		t.Fatal(err)
	}
	// The poller reads the two stores in no fixed order, so the two events
	// come in either order.
	var e, in tasksChanged
	var gotOut, gotIn bool
	waitEvent(t, ch, func(ev tasksChanged) bool {
		switch {
		case ev.Store == "me" && ev.Op == "move-out":
			e, gotOut = ev, true
		case ev.Store == "team" && ev.Op == "move-in":
			in, gotIn = ev, true
		}
		return gotOut && gotIn
	})
	if !slices.Contains(e.Removed, moved.ULID.String()) || slices.Contains(e.ULIDs, moved.ULID.String()) {
		t.Errorf("move-out event = %+v; want the task removed, not changed", e)
	}
	if !slices.Contains(in.ULIDs, moved.ULID.String()) {
		t.Errorf("move-in event = %+v; want the task changed", in)
	}
}
