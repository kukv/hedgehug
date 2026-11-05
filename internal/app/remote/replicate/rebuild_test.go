package replicate

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// reopen opens clone 1 again, as usecase does after a rebuild.
func (c *cluster) reopen() {
	c.t.Helper()
	dir := c.stores[1].Dir()
	if err := c.stores[1].Repo().Close(); err != nil {
		c.t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		c.t.Fatal(err)
	}
	c.stores[1] = s
}

// rebuild brings clone 1 onto the new history, as doctor --fix does.
func (c *cluster) rebuild() *Rebuild {
	c.t.Helper()
	b, err := RebuildStore(c.stores[1], c.opts[1])
	if err != nil {
		c.t.Fatalf("RebuildStore: %v", err)
	}
	if b == nil {
		c.t.Fatal("origin follows the clone: there is nothing to rebuild")
	}
	return b
}

func (c *cluster) commentsOf(i int, task model.ULID) []string {
	c.t.Helper()
	var out []string
	for cm, err := range c.stores[i].Comments(task) {
		if err != nil {
			c.t.Fatal(err)
		}
		out = append(out, cm.Body)
	}
	return out
}

// rewriteScene is origin rewritten without gone while clone 1, which did
// the work before it, has not synced; the sync of clone 1 stopped, and rb is
// its rebuild.
type rewriteScene struct {
	keep, gone, other *model.Task
	rb                *Rebuild
}

func newRewriteScene(t *testing.T, c *cluster, work func(sc *rewriteScene)) *rewriteScene {
	t.Helper()
	sc := &rewriteScene{}
	sc.keep = c.addTask(0, "keep")
	sc.gone = c.addTask(0, "gone")
	sc.other = c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	work(sc)
	c.rewriteOrigin(sc.gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	sc.rb = c.rebuild()
	return sc
}

// The unpushed work of the clone is put back on the new history; what it
// did to the task the new history lacks is not, and is told of; the old
// store is kept.
func TestRebuildReplaysUnpushedChanges(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(sc *rewriteScene) {
		c.set(1, sc.other.ULID, func(x *model.Task) { x.Title = "other by b" })
		c.set(1, sc.gone.ULID, func(x *model.Task) { x.Title = "gone by b" })
		c.comment(1, sc.keep.ULID, "from b", model.NewULID())
		c.comment(1, sc.gone.ULID, "to the task that is gone", model.NewULID())
	})
	if sc.rb.Replayed != 4 {
		t.Errorf("Replayed = %d, want 4", sc.rb.Replayed)
	}
	if !slices.Equal(sc.rb.Purged, []string{sc.gone.ID}) || sc.rb.PurgedTasks[0] != sc.gone.ULID {
		t.Errorf("Purged = %v %v", sc.rb.Purged, sc.rb.PurgedTasks)
	}
	if trackOf(t, c) == mustHead(t, c.stores[1]) {
		t.Fatal("setup: the clone had no unpushed work")
	}
	c.reopen()
	if p := store.RebuildPhaseIn(c.stores[1].Dir()); p != store.RebuildNone {
		t.Errorf("phase = %q", p)
	}
	if sc.rb.Kept == "" || !exists(t, sc.rb.Kept) {
		t.Errorf("Kept = %q, want a copy of the old store", sc.rb.Kept)
	}
	if got := c.task(1, sc.other.ULID); got == nil || got.Title != "other by b" {
		t.Errorf("other = %+v", got)
	}
	if c.task(1, sc.gone.ULID) != nil {
		t.Error("the task that is gone came back")
	}
	if got := c.commentsOf(1, sc.keep.ULID); !slices.Equal(got, []string{"from b"}) {
		t.Errorf("comments = %v", got)
	}
	if got := c.commentsOf(1, sc.gone.ULID); len(got) != 0 {
		t.Errorf("comments of the task that is gone = %v", got)
	}
	// One commit on the new head of origin.
	r := c.stores[1].Repo()
	origin, err := r.RemoteHead("origin")
	if err != nil {
		t.Fatal(err)
	}
	head := mustHead(t, c.stores[1])
	log, err := r.Between(head, origin)
	if err != nil || len(log) != 1 || !slices.Equal(log[0].Parents, []plumbing.Hash{origin}) {
		t.Fatalf("commits above origin = %+v, %v", log, err)
	}
	if op, _ := log[0].Trailer("Hedgehug-Op"); op != "replay" {
		t.Errorf("Hedgehug-Op = %q", op)
	}
	kinds := map[inbox.Kind]inbox.Item{}
	items, err := inbox.Open(c.stores[1]).List(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		kinds[it.Kind] = it
	}
	if it, ok := kinds[inbox.RewriteApplied]; !ok || !strings.Contains(string(it.Detail), `"replayed":4`) {
		t.Errorf("rewrite_applied = %+v", it)
	}
	if it, ok := kinds[inbox.PurgedChange]; !ok || it.Task != sc.gone.ULID {
		t.Errorf("purged_change = %+v", it)
	}
	// The next sync pushes it onto the new history.
	if r := c.sync(1); len(r.Pushed) == 0 {
		t.Errorf("Pushed = %v", r.Pushed)
	}
	if got, want := trackOf(t, c), mustHead(t, c.stores[1]); got != want {
		t.Errorf("origin is at %s, the clone at %s", got, want)
	}
}

// A clone with nothing unpushed is rebuilt as the new origin, and no old
// store is kept.
func TestRebuildWithoutUnpushedChanges(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	if sc.rb.Replayed != 0 || len(sc.rb.Purged) != 0 {
		t.Errorf("rebuild = %+v", sc.rb)
	}
	c.reopen()
	if got, want := mustHead(t, c.stores[1]), trackOf(t, c); got != want {
		t.Errorf("HEAD = %s, want the head of origin %s", got, want)
	}
	if sc.rb.Kept != "" {
		t.Errorf("Kept = %q, want none", sc.rb.Kept)
	}
	if m, err := filepath.Glob(c.stores[1].Dir() + ".kept-*"); err != nil || len(m) != 0 {
		t.Errorf("old stores kept = %v, %v", m, err)
	}
}

// Unpushed work on the tasks the new history has is all put back, and the
// old store is not kept.
func TestRebuildRemovesTheOldStoreWhenNothingWasDropped(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(sc *rewriteScene) {
		c.set(1, sc.other.ULID, func(x *model.Task) { x.Title = "other by b" })
	})
	if sc.rb.Replayed != 1 || len(sc.rb.Purged) != 0 {
		t.Fatalf("rebuild = %+v", sc.rb)
	}
	c.reopen()
	if sc.rb.Kept != "" {
		t.Errorf("Kept = %q, want none", sc.rb.Kept)
	}
	if m, err := filepath.Glob(c.stores[1].Dir() + ".*"); err != nil || len(m) != 0 {
		t.Errorf("something is left beside the store: %v, %v", m, err)
	}
	if got := c.task(1, sc.other.ULID); got == nil || got.Title != "other by b" {
		t.Errorf("other = %+v", got)
	}
}

// A task that only the unpushed commits have is not one the new history
// lacks: it is put back.
func TestRebuildKeepsATaskOnlyTheCloneHas(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	var mine *model.Task
	sc := newRewriteScene(t, c, func(*rewriteScene) {
		mine = c.addTask(1, "mine")
		c.comment(1, mine.ULID, "on mine", model.NewULID())
	})
	if len(sc.rb.Purged) != 0 {
		t.Errorf("Purged = %v", sc.rb.Purged)
	}
	c.reopen()
	if c.task(1, mine.ULID) == nil {
		t.Error("the clone's own task is lost")
	}
	if got := c.commentsOf(1, mine.ULID); !slices.Equal(got, []string{"on mine"}) {
		t.Errorf("comments = %v", got)
	}
}

// A change both the new history and this clone made to a field is decided
// as a merge decides it, and the loser is recorded.
func TestRebuildMergesConflicts(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	other := c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	c.set(1, other.ULID, func(x *model.Task) { x.Title = "b" })
	c.rewriteOrigin(gone.ULID)
	// the edit made on the new history is the later one
	i := c.cloneFresh()
	c.stores[i].SetNow(func() time.Time { return time.Now().Add(time.Hour) })
	c.set(i, other.ULID, func(x *model.Task) { x.Title = "a" })
	c.sync(i)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	c.rebuild()
	c.reopen()
	got := c.task(1, other.ULID)
	if got == nil || got.Title != "a" {
		t.Fatalf("title = %+v, want the later edit, a", got)
	}
	r := c.stores[1].Repo()
	origin, _ := r.RemoteHead("origin")
	log, err := r.Between(mustHead(t, c.stores[1]), origin)
	if err != nil || len(log) != 1 {
		t.Fatalf("log = %+v, %v", log, err)
	}
	found := false
	for _, tr := range log[0].Trailers {
		if tr.Key == "Hedgehug-Overwritten" && strings.Contains(tr.Value, " title ") && strings.Contains(tr.Value, `"b"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("no Hedgehug-Overwritten for the title b: %+v", log[0].Trailers)
	}
}

// origin that follows the clone again needs no rebuild: RebuildStore removes
// force-push.json and changes nothing.
func TestRebuildStoreWhenOriginFollowsAgain(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	old, _ := c.rewriteOrigin(gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	gitrepotest.SetRemoteMain(t, c.remote, old.String())
	b, err := RebuildStore(c.stores[1], c.opts[1])
	if err != nil || b != nil {
		t.Fatalf("RebuildStore = %+v, %v; want nothing", b, err)
	}
	if _, ok, err := ReadForcePush(c.stores[1]); err != nil || ok {
		t.Errorf("force-push.json = %v, %v", ok, err)
	}
	if p := store.RebuildPhaseIn(c.stores[1].Dir()); p != store.RebuildNone {
		t.Errorf("phase = %q", p)
	}
}

// A change git has not committed stops the rebuild.
func TestRebuildRefusesUncommittedChanges(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	keep := c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin(gone.ULID)
	p := filepath.Join(c.stores[1].Dir(), filepath.FromSlash(model.TaskPath(keep.ULID)))
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(data, "\nedited by hand\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("Run = %v, want ErrForcePush", err)
	}
	if _, err := RebuildStore(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) || !errors.Is(err, ErrDirtyRebuild) {
		t.Fatalf("RebuildStore = %v, want ErrDirtyRebuild", err)
	}
	if p := store.RebuildPhaseIn(c.stores[1].Dir()); p != store.RebuildNone {
		t.Errorf("phase = %q", p)
	}
}

func exists(t *testing.T, p string) bool {
	t.Helper()
	_, err := os.Stat(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// What was done on the new history after the force push is new to this
// clone's inbox, though the history it comes in is not the one the inbox
// followed.
func TestRebuildInboxSeesWhatCameAfterTheRewrite(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	other := c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin(gone.ULID)
	i := c.cloneFresh()
	c.set(i, other.ULID, func(x *model.Task) { x.Assignees = []string{"github.com/u1"} })
	c.sync(i)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	c.rebuild()
	c.reopen()
	c.sync(1) // the inbox catches up on the new history
	items, err := inbox.Open(c.stores[1]).List(true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(items, func(it inbox.Item) bool { return it.Kind == inbox.Assigned && it.Task == other.ULID }) {
		t.Errorf("no assigned item for the task: %+v", items)
	}
}
