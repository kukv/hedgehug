package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/views"
)

// pollServer runs the poller of a test server (real clock) and returns the
// events it publishes.
func pollServer(t *testing.T) (*testServer, <-chan sseEvent) {
	t.Helper()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	ts.srv.poller.interval = 10 * time.Millisecond
	go func() {
		defer close(done)
		ts.srv.poller.Run(ctx)
	}()
	// Wait for Run to return: a poll still writing would race the removal
	// of the temporary directory.
	t.Cleanup(func() { stop(); cancel(); <-done })
	// The first look sets the HEADs; what a test writes must come after it.
	eventually(t, "the first look of the poller", func() bool {
		ts.srv.poller.mu.Lock()
		defer ts.srv.poller.mu.Unlock()
		return len(ts.srv.poller.stores) > 0
	})
	return ts, ch
}

// settle makes the poller look twice without publishing: the first look sets
// the HEADs and the stamps, the second lets the inbox mark settle. pollMu
// keeps it from overlapping Run.
func (p *Poller) settle() {
	p.poll(false)
	p.poll(false)
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
	t.Parallel()
	ts, ch := pollServer(t)
	ts.srv.poller.settle() // the first look sets the HEADs; it sends nothing
	res, err := tasks.Add(ts.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, nil)
	if e.Store != "me" || !slices.Contains(e.ULIDs, res.ULID.String()) || e.Commit != res.Commit || e.Op != "add" {
		t.Errorf("event = %+v", e)
	}
}

// Two commits that one look finds are told one by one, each with its own
// commit and tasks: a page that made one of them knows it as its own, and the
// other is not shown as its change (or the reverse).
func TestPollTellsEachCommitOfALook(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	t.Cleanup(cancel)
	ts.srv.poller.settle()
	one, err := tasks.Add(ts.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := tasks.Add(ts.session(), tasks.AddInput{Title: "two"})
	if err != nil {
		t.Fatal(err)
	}
	ts.srv.poller.poll(true)
	first := waitEvent[tasksChanged](t, ch, nil)
	second := waitEvent[tasksChanged](t, ch, nil)
	if first.Commit != one.Commit || !slices.Equal(first.ULIDs, []string{one.ULID.String()}) || first.Op != "add" {
		t.Errorf("first event = %+v; want the commit and the task of one", first)
	}
	if second.Commit != two.Commit || !slices.Equal(second.ULIDs, []string{two.ULID.String()}) || second.Op != "add" {
		t.Errorf("second event = %+v; want the commit and the task of two", second)
	}
}

// More commits than splitMax in one look are told as one change, of HEAD.
func TestPollTellsManyCommitsAsOne(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	t.Cleanup(cancel)
	ts.srv.poller.settle()
	var last usecase.TaskResult
	for i := range splitMax + 1 {
		res, err := tasks.Add(ts.session(), tasks.AddInput{Title: fmt.Sprintf("task %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		last = res
	}
	ts.srv.poller.poll(true)
	e := waitEvent[tasksChanged](t, ch, nil)
	if e.Commit != last.Commit || len(e.ULIDs) != splitMax+1 {
		t.Errorf("event = commit %s, %d tasks; want HEAD %s and %d tasks", e.Commit, len(e.ULIDs), last.Commit, splitMax+1)
	}
}

func TestPollCommentChangesItsTask(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	res, err := tasks.Add(ts.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	ts.srv.poller.settle()
	if _, err := comments.CommentAdd(ts.session(), res.ULID.String(), "hi"); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, func(e tasksChanged) bool { return e.Op == "comment" })
	if !slices.Contains(e.ULIDs, res.ULID.String()) {
		t.Errorf("event = %+v", e)
	}
}

func TestPollRemoved(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	res, err := tasks.Add(ts.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Remove(ts.session(), res.ULID.String()); err != nil {
		t.Fatal(err)
	}
	ts.srv.poller.settle()
	if _, err := maintenance.Purge(ts.session(), 0, true); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, func(e tasksChanged) bool { return e.Op == "purge" })
	if !slices.Contains(e.Removed, res.ULID.String()) {
		t.Errorf("event = %+v", e)
	}
}

func TestPollSchemaAndViews(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	if _, err := tasks.Add(ts.session(), tasks.AddInput{Title: "one"}); err != nil {
		t.Fatal(err)
	}
	ts.srv.poller.settle()
	sc := schema.Default()
	if _, _, err := schemas.Migrate(ts.session(), schema.Migration{Target: schema.TargetStatus, Old: sc.Statuses[0].Name, New: "renamed"}, false); err != nil {
		t.Fatal(err)
	}
	waitEvent[schemaChanged](t, ch, nil)
	layout := "table"
	if _, err := views.ViewSave(ts.session(), "v", views.ViewChange{Layout: &layout}); err != nil {
		t.Fatal(err)
	}
	waitEvent[viewsChanged](t, ch, nil)
}

func TestPollSyncState(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	ts.srv.poller.settle()
	// push_at is told for a store the loop has synced.
	ts.srv.loop.Synced("me")
	s, err := ts.session().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := replicate.MarkPending(s, time.Now()); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[usecase.SyncStatusView](t, ch, func(e usecase.SyncStatusView) bool { return e.PendingSince != nil })
	if e.Store != "me" {
		t.Errorf("event = %+v", e)
	}
	ts.srv.loop.mu.Lock()
	last := ts.srv.loop.last["me"]
	ts.srv.loop.mu.Unlock()
	if want := maxTime(*e.PendingSince, last).Add(30 * time.Second); e.PushAt == nil || !e.PushAt.Equal(want) {
		t.Errorf("push_at = %v; want %v (30s after the later of pending_since and the last sync)", e.PushAt, want)
	}
	if e.Interval <= 0 || e.PushInterval <= 0 {
		t.Errorf("interval = %v, push_interval = %v; want the sync.* intervals in seconds", e.Interval, e.PushInterval)
	}
}

func TestPollUnread(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	res, err := tasks.Add(ts.session(), tasks.AddInput{Title: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		t.Fatal(err)
	}
	ts.srv.poller.settle()
	nid := ts.notifyMe(overwrittenItem(res.ULID, v.ID, `"first"`))
	waitEvent[unreadEvent](t, ch, func(e unreadEvent) bool { return e.Count == 1 })
	// hh inbox read in another process: no HEAD moves, the files of the
	// inbox do.
	if _, err := notifications.InboxRead(ts.session(), []string{nid}, false); err != nil {
		t.Fatal(err)
	}
	waitEvent[unreadEvent](t, ch, func(e unreadEvent) bool { return e.Count == 0 })
}

func TestPollFirstLookSendsNothing(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.addRemote()
	ch, cancel := ts.srv.hub.subscribe()
	defer cancel()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ts.srv.poller.Run(ctx)
	}()
	defer func() { stop(); <-done }()
	eventually(t, "the first look", func() bool {
		ts.srv.poller.mu.Lock()
		defer ts.srv.poller.mu.Unlock()
		return ts.srv.poller.looked
	})
	ts.srv.poller.poll(true)
	ts.srv.poller.poll(true)
	select {
	case e := <-ch:
		t.Errorf("event on the first looks: %+v", e)
	default:
	}
}

func TestPollInboxStampIsStable(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	res, err := tasks.Add(ts.session(), tasks.AddInput{Title: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		t.Fatal(err)
	}
	ts.notifyMe(overwrittenItem(res.ULID, v.ID, `"first"`))
	ts.srv.poller.poll(false)
	ts.srv.poller.poll(false) // the Catch of the first moved the mark; it settles here
	s, err := ts.session().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before := inbox.Open(s).Stamp()
	if _, _, err := notifications.InboxList(ts.session(), false); err != nil {
		t.Fatal(err)
	}
	if after := inbox.Open(s).Stamp(); after != before {
		t.Errorf("InboxList changed the files: %q -> %q", before, after)
	}
}

func TestChildTaskIsCommentsOnly(t *testing.T) {
	t.Parallel()
	task, other := model.NewULID(), model.NewULID()
	if u, ok := childTask(model.CommentPath(task, other)); !ok || u != task {
		t.Errorf("comment path: %v %v", u, ok)
	}
	if _, ok := childTask(model.WorklogPath(task, other)); ok {
		t.Error("a worklog path changes no task")
	}
}

func TestPollNewStoreTellsTheClients(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	ts.srv.poller.settle()
	// A store made after the first look: its first commits may come before the poller sees it.
	if _, err := stores.StoreInit(ts.session(), stores.StoreInitInput{Name: "later", Prefix: "LT", Workspace: "later"}); err != nil {
		t.Fatal(err)
	}
	e := waitEvent(t, ch, func(e tasksChanged) bool { return e.Store == "later" })
	if e.Commit == "" {
		t.Error("the event of a new store has no commit; it says HEAD now")
	}
	waitEvent(t, ch, func(e usecase.SyncStatusView) bool { return e.Store == "later" })
}

func TestPollStoreThatComesBackTellsTheClients(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	ts.srv.poller.settle()
	if _, err := stores.StoreInit(ts.session(), stores.StoreInitInput{Name: "later", Prefix: "LT", Workspace: "later"}); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, ch, func(e tasksChanged) bool { return e.Store == "later" })
	ts.srv.poller.settle()

	c, _, err := ts.session().Config()
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Select("later")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stores.StoreRemove(ts.session(), "later", true); err != nil {
		t.Fatal(err)
	}
	ts.srv.poller.settle() // the poller forgets the store
	c, _, err = ts.session().Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("later", e.Path, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	// What happened while it was away is not known: the clients read it again.
	ts.srv.poller.poll(true)
	e2 := waitEvent(t, ch, func(e tasksChanged) bool { return e.Store == "later" })
	if e2.Commit == "" {
		t.Error("the event of a store that came back has no commit")
	}
}

// The server starts with no store (Onboarding): a poll that finds none has looked, and the first
// store made after it tells the clients, though it may already hold the first task.
func TestPollFirstStoreAfterNoStoreTellsTheClients(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	if _, err := stores.StoreRemove(ts.session(), "me", true); err != nil {
		t.Fatal(err)
	}
	ts.srv.poller = newPoller(ts.srv, ts.srv.hub)
	ch, cancel := ts.srv.hub.subscribe()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	ts.srv.poller.interval = 10 * time.Millisecond
	go func() {
		defer close(done)
		ts.srv.poller.Run(ctx)
	}()
	t.Cleanup(func() { stop(); cancel(); <-done })
	eventually(t, "the look of the poller at no store", func() bool {
		ts.srv.poller.mu.Lock()
		defer ts.srv.poller.mu.Unlock()
		return ts.srv.poller.scanned
	})
	if _, err := stores.StoreInit(ts.session(), stores.StoreInitInput{Name: "first", Prefix: "FT", Workspace: "first"}); err != nil {
		t.Fatal(err)
	}
	e := waitEvent(t, ch, func(e tasksChanged) bool { return e.Store == "first" })
	if e.Commit == "" {
		t.Error("the event has no commit")
	}
}

func TestPollFailedFirstLookIsNotTheFirstLook(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	defer cancel()
	cfg := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	good, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(": not yaml ["), 0o600); err != nil {
		t.Fatal(err)
	}
	ts.srv.poller.poll(true) // returns early: the stores were not read
	if err := os.WriteFile(cfg, good, 0o600); err != nil {
		t.Fatal(err)
	}
	ts.srv.poller.poll(true) // the first look of the stores: it sends nothing
	select {
	case e := <-ch:
		t.Errorf("event on the first look of the stores: %+v", e)
	default:
	}
}
