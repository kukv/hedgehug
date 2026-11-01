package server

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// pollServer runs the poller of a test server (real clock) and returns the
// events it publishes.
func pollServer(t *testing.T) (*testServer, <-chan sseEvent) {
	t.Helper()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.Subscribe()
	ctx, stop := context.WithCancel(context.Background())
	go ts.srv.poller.Run(ctx)
	t.Cleanup(func() { stop(); cancel() })
	// The first look sets the HEADs; what a test writes must come after it.
	eventually(t, "the first look of the poller", func() bool {
		ts.srv.poller.mu.Lock()
		defer ts.srv.poller.mu.Unlock()
		return len(ts.srv.poller.stores) > 0
	})
	return ts, ch
}

// waitEvent returns the first event of type T, skipping the others.
func waitEvent[T any](t *testing.T, ch <-chan sseEvent, match func(T) bool) T {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatal("the hub dropped the subscription")
			}
			if v, ok := e.(T); ok && (match == nil || match(v)) {
				return v
			}
		case <-timeout:
			var zero T
			t.Fatalf("no %T event", zero)
			return zero
		}
	}
}

func TestPollTasksChanged(t *testing.T) {
	ts, ch := pollServer(t)
	time.Sleep(2 * pollInterval) // the first look sets the HEADs; it sends nothing
	res, err := ts.session().Add(ops.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, nil)
	if e.Store != "me" || !slices.Contains(e.ULIDs, res.ULID.String()) || e.Commit != res.Commit || e.Op != "add" {
		t.Errorf("event = %+v", e)
	}
}

func TestPollCommentChangesItsTask(t *testing.T) {
	ts, ch := pollServer(t)
	res, err := ts.session().Add(ops.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * pollInterval)
	if _, err := ts.session().CommentAdd(res.ULID.String(), "hi"); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, func(e tasksChanged) bool { return e.Op == "comment" })
	if !slices.Contains(e.ULIDs, res.ULID.String()) {
		t.Errorf("event = %+v", e)
	}
}

func TestPollRemoved(t *testing.T) {
	ts, ch := pollServer(t)
	res, err := ts.session().Add(ops.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.session().Remove(res.ULID.String()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * pollInterval)
	if _, err := ts.session().PurgeUnchecked(res.ULID.String()); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, func(e tasksChanged) bool { return e.Op == "purge" })
	if !slices.Contains(e.Removed, res.ULID.String()) {
		t.Errorf("event = %+v", e)
	}
}

func TestPollSchemaAndViews(t *testing.T) {
	ts, ch := pollServer(t)
	if _, err := ts.session().Add(ops.AddInput{Title: "one"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * pollInterval)
	sc := schema.Default()
	if _, _, err := ts.session().Migrate(schema.Migration{Target: schema.TargetStatus, Old: sc.Statuses[0].Name, New: "renamed"}, false); err != nil {
		t.Fatal(err)
	}
	waitEvent[schemaChanged](t, ch, nil)
	layout := "table"
	if _, err := ts.session().ViewSave("v", ops.ViewChange{Layout: &layout}); err != nil {
		t.Fatal(err)
	}
	waitEvent[viewsChanged](t, ch, nil)
}

func TestPollSyncState(t *testing.T) {
	ts, ch := pollServer(t)
	time.Sleep(2 * pollInterval)
	s, err := ts.session().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := hhsync.MarkPending(s, time.Now()); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[syncStatus](t, ch, func(e syncStatus) bool { return e.PendingSince != nil })
	if e.Store != "me" {
		t.Errorf("event = %+v", e)
	}
}

func TestPollUnread(t *testing.T) {
	ts, ch := pollServer(t)
	res, err := ts.session().Add(ops.AddInput{Title: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * pollInterval)
	nid := ts.notifyMe(overwrittenItem(res.ULID, v.ID, "title", `"first"`))
	waitEvent[unreadEvent](t, ch, func(e unreadEvent) bool { return e.Count == 1 })
	// hh inbox read in another process: no HEAD moves, the files of the
	// inbox do.
	if _, err := ts.session().InboxRead([]string{nid}, false); err != nil {
		t.Fatal(err)
	}
	waitEvent[unreadEvent](t, ch, func(e unreadEvent) bool { return e.Count == 0 })
}
