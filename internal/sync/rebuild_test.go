package sync

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// swap puts the rebuild of res in the place of clone 1, as ops does after
// it released the locks, and opens the store again.
func (c *cluster) swap(res Result) {
	c.t.Helper()
	dir := c.stores[1].Dir()
	if err := c.stores[1].Repo().Close(); err != nil {
		c.t.Fatal(err)
	}
	if err := Swap(dir, res.Rebuild); err != nil {
		c.t.Fatalf("Swap: %v", err)
	}
	s, err := store.Open(dir)
	if err != nil {
		c.t.Fatal(err)
	}
	c.stores[1] = s
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

// rewriteScene is a rewrite by clone 0 that drops gone, found by the sync of
// clone 1, which did the work before it.
type rewriteScene struct {
	keep, gone, other *model.Task
	rec               model.Rewrite
	res               Result
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
	_, sc.rec = c.rewriteLocally(sc.gone.ULID, plumbing.ZeroHash, true)
	c.forcePush()
	sc.res = c.sync(1)
	if sc.res.Rebuild == nil {
		t.Fatal("the sync made no rebuild")
	}
	return sc
}

// The unpushed work of the clone is put back on the new history; what it
// did to the purged task is not, and is told of.
func TestRebuildReplaysUnpushedChanges(t *testing.T) {
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(sc *rewriteScene) {
		c.set(1, sc.other.ULID, func(x *model.Task) { x.Title = "other by b" })
		c.set(1, sc.gone.ULID, func(x *model.Task) { x.Title = "gone by b" })
		c.comment(1, sc.keep.ULID, "from b", model.NewULID())
		c.comment(1, sc.gone.ULID, "to the purged task", model.NewULID())
	})
	if sc.res.Rebuild.Replayed != 4 {
		t.Errorf("Replayed = %d, want 4", sc.res.Rebuild.Replayed)
	}
	if !slices.Equal(sc.res.Rebuild.Purged, []string{c.task(1, sc.gone.ULID).ID}) || sc.res.Rebuild.PurgedTasks[0] != sc.gone.ULID {
		t.Errorf("Purged = %v %v", sc.res.Rebuild.Purged, sc.res.Rebuild.PurgedTasks)
	}
	newHead := trackOf(t, c) // the tracking ref is as before the fetch: the old origin
	if newHead == mustHead(t, c.stores[1]) {
		t.Fatal("setup: the clone had no unpushed work")
	}
	c.swap(sc.res)
	if left, err := FindLeftover(c.stores[1].Dir()); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
	if got := c.task(1, sc.other.ULID); got == nil || got.Title != "other by b" {
		t.Errorf("other = %+v", got)
	}
	if c.task(1, sc.gone.ULID) != nil {
		t.Error("the purged task came back")
	}
	if got := c.commentsOf(1, sc.keep.ULID); !slices.Equal(got, []string{"from b"}) {
		t.Errorf("comments = %v", got)
	}
	if got := c.commentsOf(1, sc.gone.ULID); len(got) != 0 {
		t.Errorf("comments of the purged task = %v", got)
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
	applied, err := ReadApplied(c.stores[1])
	if err != nil || len(applied) != 1 || !applied[0].Equal(sc.rec.At) {
		t.Errorf("applied = %v, %v", applied, err)
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
	// The next sync pushes it; the clone that rewrote gets it.
	if r := c.sync(1); len(r.Pushed) == 0 {
		t.Errorf("Pushed = %v", r.Pushed)
	}
	c.sync(0)
	if got := c.task(0, sc.other.ULID); got == nil || got.Title != "other by b" {
		t.Errorf("clone 0: other = %+v", got)
	}
	if c.task(0, sc.gone.ULID) != nil {
		t.Error("clone 0 has the purged task")
	}
}

// A clone with nothing unpushed is rebuilt as the new origin.
func TestRebuildWithoutUnpushedChanges(t *testing.T) {
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	if sc.res.Rebuild.Replayed != 0 || len(sc.res.Rebuild.Purged) != 0 {
		t.Errorf("rebuild = %+v", sc.res.Rebuild)
	}
	c.swap(sc.res)
	if got, want := mustHead(t, c.stores[1]), mustHead(t, c.stores[0]); got != want {
		t.Errorf("HEAD = %s, want the head of origin %s", got, want)
	}
}

// A change both the rewriting clone and this one made to a field is decided
// as a merge decides it, and the loser is recorded.
func TestRebuildMergesConflicts(t *testing.T) {
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	other := c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	c.set(1, other.ULID, func(x *model.Task) { x.Title = "b" })
	c.rewriteLocally(gone.ULID, plumbing.ZeroHash, true)
	// clone 0's edit is the later one
	c.stores[0].SetNow(func() time.Time { return time.Now().Add(time.Hour) })
	c.set(0, other.ULID, func(x *model.Task) { x.Title = "a" })
	c.forcePush()
	res := c.sync(1)
	if res.Rebuild == nil {
		t.Fatal("no rebuild")
	}
	c.swap(res)
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

// A write that lands between the rebuild and the swap stops the swap.
func TestSwapStale(t *testing.T) {
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	late := c.addTask(1, "late")
	dir := c.stores[1].Dir()
	if err := c.stores[1].Repo().Close(); err != nil {
		t.Fatal(err)
	}
	if err := Swap(dir, sc.res.Rebuild); !errors.Is(err, ErrRebuildStale) {
		t.Fatalf("Swap = %v, want ErrRebuildStale", err)
	}
	if _, err := os.Stat(sc.res.Rebuild.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the rebuild is still there: %v", err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.stores[1] = s
	// The next sync makes it again, and the late task is put back.
	res := c.sync(1)
	if res.Rebuild == nil || res.Rebuild.Replayed != 1 {
		t.Fatalf("second rebuild = %+v", res.Rebuild)
	}
	c.swap(res)
	if c.task(1, late.ULID) == nil {
		t.Error("the late task is lost")
	}
}

// A change git has not committed stops the swap too, and the rebuild.
func TestRebuildRefusesUncommittedChanges(t *testing.T) {
	c := newCluster(t, 2)
	keep := c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	c.rewriteLocally(gone.ULID, plumbing.ZeroHash, true)
	c.forcePush()
	p := filepath.Join(c.stores[1].Dir(), filepath.FromSlash(model.TaskPath(keep.ULID)))
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(data, "\nedited by hand\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) || !errors.Is(err, ErrDirtyRebuild) {
		t.Fatalf("Run = %v, want ErrDirtyRebuild", err)
	}
	if left, err := FindLeftover(c.stores[1].Dir()); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
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

// A swap that stopped after the old store was moved away is finished.
func TestFinishLeftoverNoStore(t *testing.T) {
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	dir := c.stores[1].Dir()
	if err := c.stores[1].Repo().Close(); err != nil {
		t.Fatal(err)
	}
	before := dir + ".before-rewrite-20260101T000000.000000000"
	if err := os.Rename(dir, before); err != nil {
		t.Fatal(err)
	}
	l, err := FindLeftover(dir)
	if err != nil || l.New != sc.res.Rebuild.Dir || !slices.Equal(l.Before, []string{before}) {
		t.Fatalf("leftover = %+v, %v", l, err)
	}
	if _, err := store.Open(dir); err == nil {
		t.Fatal("a store without a directory opens")
	}
	if err := FinishLeftover(dir, l); err != nil {
		t.Fatal(err)
	}
	if exists(t, before) || exists(t, l.New) {
		t.Error("something is left")
	}
	if _, err := store.Open(dir); err != nil {
		t.Errorf("the store does not open: %v", err)
	}
}

// With the new store in place, the old one is removed; a rebuild never
// swapped is removed; with only the old one left, it is put back.
func TestFinishLeftoverOtherStates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "s")
	mk := func(p string) {
		if err := os.MkdirAll(p, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "f"), []byte(filepath.Base(p)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mk(dir)
	mk(dir + ".before-rewrite-1")
	mk(dir + ".rewrite-new")
	l, err := FindLeftover(dir)
	if err != nil || l.None() {
		t.Fatalf("leftover = %+v, %v", l, err)
	}
	if err := FinishLeftover(dir, l); err != nil {
		t.Fatal(err)
	}
	if l, err := FindLeftover(dir); err != nil || !l.None() || !exists(t, dir) {
		t.Errorf("leftover = %+v, %v", l, err)
	}
	// only the old one: put back
	if err := os.Rename(dir, dir+".before-rewrite-2"); err != nil {
		t.Fatal(err)
	}
	l, _ = FindLeftover(dir)
	if err := FinishLeftover(dir, l); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "f"))
	if err != nil || string(b) != "s" {
		t.Errorf("the store = %q, %v", b, err)
	}
	if l, _ := FindLeftover(dir); !l.None() {
		t.Errorf("leftover = %+v", l)
	}
}

// The neighbours of other names are not a leftover.
func TestFindLeftoverIgnoresOtherNames(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "s")
	for _, n := range []string{"s", "s2.rewrite-new", "s2.before-rewrite-1", "t.rewrite-new"} {
		if err := os.MkdirAll(filepath.Join(root, n), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if l, err := FindLeftover(dir); err != nil || !l.None() {
		t.Errorf("leftover = %+v, %v", l, err)
	}
	if l, err := FindLeftover(filepath.Join(root, "missing", "s")); err != nil || !l.None() {
		t.Errorf("leftover of a missing parent = %+v, %v", l, err)
	}
}

// What the other clone did after the rewrite is new to this clone's inbox,
// though the history it comes in is not the one the inbox followed.
func TestRebuildInboxSeesWhatCameAfterTheRewrite(t *testing.T) {
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	other := c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	c.rewriteLocally(gone.ULID, plumbing.ZeroHash, true)
	c.set(0, other.ULID, func(x *model.Task) { x.Assignees = []string{"github.com/u1"} })
	c.forcePush()
	res := c.sync(1)
	if res.Rebuild == nil {
		t.Fatal("no rebuild")
	}
	c.swap(res)
	c.sync(1) // the inbox catches up on the new history
	items, err := inbox.Open(c.stores[1]).List(true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(items, func(it inbox.Item) bool { return it.Kind == inbox.Assigned && it.Task == other.ULID }) {
		t.Errorf("no assigned item for the task: %+v", items)
	}
}
