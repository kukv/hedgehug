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
	time.Sleep(2 * pollInterval)

	if _, err := ts.session().MoveStore(ops.MoveStoreInput{Arg: moved.ULID.String(), To: "team", Yes: true}); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, func(e tasksChanged) bool { return e.Store == "me" && e.Op == "move-out" })
	if !slices.Contains(e.Removed, moved.ULID.String()) || slices.Contains(e.ULIDs, moved.ULID.String()) {
		t.Errorf("move-out event = %+v; want the task removed, not changed", e)
	}
	in := waitEvent[tasksChanged](t, ch, func(e tasksChanged) bool { return e.Store == "team" && e.Op == "move-in" })
	if !slices.Contains(in.ULIDs, moved.ULID.String()) {
		t.Errorf("move-in event = %+v; want the task changed", in)
	}
}
