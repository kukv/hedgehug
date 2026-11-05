package watch_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/views"
	"github.com/kukv/hedgehug/internal/app/usecase/watch"
	"github.com/kukv/hedgehug/internal/lib/testenv"
)

func TestPollTasksChanged(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	f.settle() // the first look sets the HEADs; it sends nothing
	res, err := tasks.Add(f.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	e := waitEvent[watch.Change](t, f.ch, nil)
	if e.Store != "me" || !slices.Contains(e.ULIDs, res.ULID.String()) || e.Commit != res.Commit || e.Op != "add" {
		t.Errorf("event = %+v", e)
	}
}

// Two commits that one look finds are told one by one, each with its own
// commit and tasks: a page that made one of them knows it as its own, and the
// other is not shown as its change (or the reverse).
func TestPollTellsEachCommitOfALook(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.settle()
	one, err := tasks.Add(f.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := tasks.Add(f.session(), tasks.AddInput{Title: "two"})
	if err != nil {
		t.Fatal(err)
	}
	f.w.Poll(true)
	first := waitEvent[watch.Change](t, f.ch, nil)
	second := waitEvent[watch.Change](t, f.ch, nil)
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
	f := newFixture(t)
	f.settle()
	var last usecase.TaskResult
	for i := range watch.SplitMax + 1 {
		res, err := tasks.Add(f.session(), tasks.AddInput{Title: fmt.Sprintf("task %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		last = res
	}
	f.w.Poll(true)
	e := waitEvent[watch.Change](t, f.ch, nil)
	if e.Commit != last.Commit || len(e.ULIDs) != watch.SplitMax+1 {
		t.Errorf("event = commit %s, %d tasks; want HEAD %s and %d tasks", e.Commit, len(e.ULIDs), last.Commit, watch.SplitMax+1)
	}
}

func TestPollCommentChangesItsTask(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	res, err := tasks.Add(f.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	f.settle()
	if _, err := comments.CommentAdd(f.session(), res.ULID.String(), "hi"); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[watch.Change](t, f.ch, func(e watch.Change) bool { return e.Op == "comment" })
	if !slices.Contains(e.ULIDs, res.ULID.String()) {
		t.Errorf("event = %+v", e)
	}
}

func TestPollRemoved(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	res, err := tasks.Add(f.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Remove(f.session(), res.ULID.String()); err != nil {
		t.Fatal(err)
	}
	f.settle()
	if _, err := maintenance.Purge(f.session(), 0, true); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[watch.Change](t, f.ch, func(e watch.Change) bool { return e.Op == "purge" })
	if !slices.Contains(e.Removed, res.ULID.String()) {
		t.Errorf("event = %+v", e)
	}
}

func TestPollSchemaAndViews(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	if _, err := tasks.Add(f.session(), tasks.AddInput{Title: "one"}); err != nil {
		t.Fatal(err)
	}
	f.settle()
	sc := schema.Default()
	if _, err := schemas.Migrate(f.session(), schema.Migration{Target: schema.TargetStatus, Old: sc.Statuses[0].Name, New: "renamed"}, false); err != nil {
		t.Fatal(err)
	}
	waitEvent[schemaChanged](t, f.ch, nil)
	layout := "table"
	if _, err := views.ViewSave(f.session(), "v", views.ViewChange{Layout: &layout}); err != nil {
		t.Fatal(err)
	}
	waitEvent[viewsChanged](t, f.ch, nil)
}

func TestPollUnread(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	res, err := tasks.Add(f.session(), tasks.AddInput{Title: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		t.Fatal(err)
	}
	f.settle()
	nid := f.notifyMe(overwrittenItem(res.ULID, v.ID, `"first"`))
	waitEvent[unreadEvent](t, f.ch, func(e unreadEvent) bool { return e.Count == 1 })
	// hh inbox read in another process: no HEAD moves, the files of the
	// inbox do.
	if _, err := notifications.InboxRead(f.session(), []string{nid}, false); err != nil {
		t.Fatal(err)
	}
	waitEvent[unreadEvent](t, f.ch, func(e unreadEvent) bool { return e.Count == 0 })
}

func TestPollFirstLookSendsNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	usecasetest.AddBareRemote(t, f.ts)
	f.run()
	testenv.WaitFor(t, "the first look", func() bool { return watch.Looked(f.w) })
	f.w.Poll(true)
	f.w.Poll(true)
	select {
	case e := <-f.ch:
		t.Errorf("event on the first looks: %+v", e)
	default:
	}
}

func TestPollInboxStampIsStable(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res, err := tasks.Add(f.session(), tasks.AddInput{Title: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		t.Fatal(err)
	}
	f.notifyMe(overwrittenItem(res.ULID, v.ID, `"first"`))
	f.w.Poll(false)
	f.w.Poll(false) // the Catch of the first moved the mark; it settles here
	s, err := f.session().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before := inbox.Open(s).Stamp()
	if _, _, err := notifications.InboxList(f.session(), false); err != nil {
		t.Fatal(err)
	}
	if after := inbox.Open(s).Stamp(); after != before {
		t.Errorf("InboxList changed the files: %q -> %q", before, after)
	}
}

func TestChildTaskIsCommentsAndWorklogs(t *testing.T) {
	t.Parallel()
	task, other := model.NewULID(), model.NewULID()
	if u, ok := watch.ChildTask(model.CommentPath(task, other)); !ok || u != task {
		t.Errorf("comment path: %v %v", u, ok)
	}
	// The detail of a task shows the time spent.
	if u, ok := watch.ChildTask(model.WorklogPath(task, other)); !ok || u != task {
		t.Errorf("worklog path: %v %v", u, ok)
	}
	if _, ok := watch.ChildTask(model.SprintPath(other)); ok {
		t.Error("a sprint is of no task")
	}
}

func TestPollNewStoreTellsTheClients(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	f.settle()
	// A store made after the first look: its first commits may come before the watcher sees it.
	if _, err := stores.StoreInit(f.session(), stores.StoreInitInput{Name: "later", Prefix: "LT", Workspace: "later"}); err != nil {
		t.Fatal(err)
	}
	e := waitEvent(t, f.ch, func(e watch.Change) bool { return e.Store == "later" })
	if e.Commit == "" {
		t.Error("the event of a new store has no commit; it says HEAD now")
	}
	waitEvent(t, f.ch, func(e usecase.SyncStatusView) bool { return e.Store == "later" })
}

// The server makes tasks.changed's JSON from the event: a nil list would be
// null there, so a change that names no task still carries empty lists.
func TestPollChangeWithoutTasksHasEmptyLists(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	f.settle()
	if _, err := stores.StoreInit(f.session(), stores.StoreInitInput{Name: "later", Prefix: "LT", Workspace: "later"}); err != nil {
		t.Fatal(err)
	}
	e := waitEvent(t, f.ch, func(e watch.Change) bool { return e.Store == "later" })
	if len(e.ULIDs) != 0 || len(e.Removed) != 0 {
		t.Fatalf("event = %+v; want no task named", e)
	}
	if e.ULIDs == nil || e.Removed == nil {
		t.Errorf("ULIDs nil = %v, Removed nil = %v; want both empty and not nil", e.ULIDs == nil, e.Removed == nil)
	}
}

func TestPollStoreThatComesBackTellsTheClients(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	f.settle()
	if _, err := stores.StoreInit(f.session(), stores.StoreInitInput{Name: "later", Prefix: "LT", Workspace: "later"}); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, f.ch, func(e watch.Change) bool { return e.Store == "later" })
	f.settle()

	c, _, err := f.session().Config()
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Select("later")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stores.StoreRemove(f.session(), "later", true); err != nil {
		t.Fatal(err)
	}
	f.settle() // the watcher forgets the store
	c, _, err = f.session().Config()
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
	f.w.Poll(true)
	e2 := waitEvent(t, f.ch, func(e watch.Change) bool { return e.Store == "later" })
	if e2.Commit == "" {
		t.Error("the event of a store that came back has no commit")
	}
}

// The server starts with no store (Onboarding): a poll that finds none has looked, and the first
// store made after it tells the clients, though it may already hold the first task.
func TestPollFirstStoreAfterNoStoreTellsTheClients(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if _, err := stores.StoreRemove(f.session(), "me", true); err != nil {
		t.Fatal(err)
	}
	f.w = f.newWatcher()
	f.run()
	testenv.WaitFor(t, "the look of the watcher at no store", func() bool { return watch.Scanned(f.w) })
	if _, err := stores.StoreInit(f.session(), stores.StoreInitInput{Name: "first", Prefix: "FT", Workspace: "first"}); err != nil {
		t.Fatal(err)
	}
	e := waitEvent(t, f.ch, func(e watch.Change) bool { return e.Store == "first" })
	if e.Commit == "" {
		t.Error("the event has no commit")
	}
}

func TestPollFailedFirstLookIsNotTheFirstLook(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	cfg := filepath.Join(f.ts.Env["HEDGEHUG_HOME"], "config.yaml")
	good, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(": not yaml ["), 0o600); err != nil {
		t.Fatal(err)
	}
	f.w.Poll(true) // returns early: the stores were not read
	if err := os.WriteFile(cfg, good, 0o600); err != nil {
		t.Fatal(err)
	}
	f.w.Poll(true) // the first look of the stores: it sends nothing
	select {
	case e := <-f.ch:
		t.Errorf("event on the first look of the stores: %+v", e)
	default:
	}
}
