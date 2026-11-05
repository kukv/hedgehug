package replicate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// stoppedScene is clone 1 with unpushed work, right after prepare wrote
// rebuild.json: nothing of the store has changed yet.
type stoppedScene struct {
	rec        *rebuildRecord
	b          *Rebuild
	mine, gone *model.Task
	other      *model.Task
	paths      []string
}

func newStoppedScene(t *testing.T, c *cluster, dropped bool) *stoppedScene {
	t.Helper()
	sc := &stoppedScene{}
	c.addTask(0, "keep")
	sc.gone = c.addTask(0, "gone")
	sc.other = c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	sc.mine = c.addTask(1, "mine")
	c.set(1, sc.other.ULID, func(x *model.Task) { x.Title = "other by b" })
	if dropped {
		c.set(1, sc.gone.ULID, func(x *model.Task) { x.Title = "gone by b" })
	}
	c.rewriteOrigin(sc.gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	held, err := store.LockInOrder(c.stores[1])
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if sc.rec, sc.b, err = prepare(c.stores[1], c.opts[1]); err != nil || sc.rec == nil {
		t.Fatalf("prepare = %+v, %v", sc.rec, err)
	}
	r := c.stores[1].Repo()
	ft, _ := r.TreeOf(plumbing.NewHash(sc.rec.OldHead))
	tt, _ := r.TreeOf(plumbing.NewHash(sc.rec.Target))
	changes, err := r.DiffTrees(ft, tt, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range changes {
		sc.paths = append(sc.paths, ch.Path)
	}
	return sc
}

// checkRebuilt is what a finished rebuild leaves, however it got there.
func checkRebuilt(t *testing.T, c *cluster, sc *stoppedScene) {
	t.Helper()
	dir := c.stores[1].Dir()
	if p := store.RebuildPhaseIn(dir); p != store.RebuildNone {
		t.Errorf("phase = %q", p)
	}
	c.reopen()
	s := c.stores[1]
	if got := mustHead(t, s); got.String() != sc.rec.Target {
		t.Errorf("HEAD = %s, want %s", got, sc.rec.Target)
	}
	if got, err := s.Repo().RemoteHead("origin"); err != nil || got.String() != sc.rec.Origin {
		t.Errorf("origin = %s, %v; want %s", got, err, sc.rec.Origin)
	}
	if st, err := s.Repo().Status(); err != nil || len(st) != 0 {
		t.Errorf("Status = %v, %v", st, err)
	}
	if _, ok, err := ReadForcePush(s); err != nil || ok {
		t.Errorf("force-push.json = %v, %v", ok, err)
	}
	if got := c.task(1, sc.mine.ULID); got == nil {
		t.Error("the unpushed task is lost")
	}
	if got := c.task(1, sc.other.ULID); got == nil || got.Title != "other by b" {
		t.Errorf("other = %+v", got)
	}
	if c.task(1, sc.gone.ULID) != nil {
		t.Error("the task that is gone came back")
	}
	items, err := inbox.Open(s).List(true)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, it := range items {
		if it.Kind == inbox.RewriteApplied {
			n++
		}
	}
	if n != 1 {
		t.Errorf("rewrite_applied items = %d, want 1: %+v", n, items)
	}
	if err := s.Repo().Verify(); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

// However far a rebuild got, FinishRebuild ends in the same place.
func TestFinishRebuildFromEachStep(t *testing.T) {
	t.Parallel()
	for name, stop := range map[string]func(t *testing.T, c *cluster, sc *stoppedScene){
		"nothing done": func(*testing.T, *cluster, *stoppedScene) {},
		"one file written": func(t *testing.T, c *cluster, sc *stoppedScene) {
			p := model.TaskPath(sc.mine.ULID)
			b, err := c.stores[1].Repo().ReadAt(plumbing.NewHash(sc.rec.Target), p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(c.stores[1].Dir(), filepath.FromSlash(p)), b, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"main moved": func(t *testing.T, c *cluster, sc *stoppedScene) {
			if err := c.stores[1].Repo().MoveMain(plumbing.NewHash(sc.rec.OldHead), plumbing.NewHash(sc.rec.Target), sc.paths); err != nil {
				t.Fatal(err)
			}
		},
		"applied, phase not written": func(t *testing.T, c *cluster, sc *stoppedScene) {
			if err := apply(c.stores[1], sc.rec, &Rebuild{}); err != nil {
				t.Fatal(err)
			}
		},
		"applied, phase written": func(t *testing.T, c *cluster, sc *stoppedScene) {
			if err := apply(c.stores[1], sc.rec, &Rebuild{}); err != nil {
				t.Fatal(err)
			}
			sc.rec.Phase = store.RebuildPrune
			if err := writeRecord(c.stores[1], sc.rec); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := newCluster(t, 2)
			sc := newStoppedScene(t, c, false)
			stop(t, c, sc)
			c.reopen() // a new process
			b, err := FinishRebuild(c.stores[1])
			if err != nil || b == nil {
				t.Fatalf("FinishRebuild = %+v, %v", b, err)
			}
			checkRebuilt(t, c, sc)
			// And again: nothing is left to do.
			if b, err := FinishRebuild(c.stores[1]); err != nil || b != nil {
				t.Errorf("second FinishRebuild = %+v, %v", b, err)
			}
		})
	}
}

// The old history is gone from the disk with the rebuild; nothing was kept.
func TestRebuildRemovesTheOldHistory(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newStoppedScene(t, c, false)
	if b, err := FinishRebuild(c.stores[1]); err != nil || b == nil {
		t.Fatalf("FinishRebuild = %+v, %v", b, err)
	}
	r, err := gitrepo.Open(c.stores[1].Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if _, err := r.TreeOf(plumbing.NewHash(sc.rec.OldHead)); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("the old head reads: %v", err)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("Verify: %v", err)
	}
	// Real git reads what the prune left. The CI job without git has none.
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Log("git is not installed: git fsck is not run")
		return
	}
	cmd := exec.Command(gitBin, "-C", c.stores[1].Dir(), "fsck", "--full", "--strict")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("git fsck: %v\n%s", err, out)
	}
}

// With changes that cannot be put back, a copy of the store as it was is
// kept, with the old history, and a copy cut short is cleared away.
func TestRebuildKeepsACopyOfTheOldStore(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	if err := os.MkdirAll(c.stores[1].Dir()+keptNew, 0o750); err != nil {
		t.Fatal(err)
	}
	// The temporary file of a write that another process has open is no part
	// of the store either.
	localDir := filepath.Join(c.stores[1].Dir(), filepath.FromSlash(store.LocalDir))
	if err := os.MkdirAll(localDir, 0o750); err != nil {
		t.Fatal(err)
	}
	tmpName := ".sync.json.tmp-123"
	if err := os.WriteFile(filepath.Join(localDir, tmpName), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sc := newStoppedScene(t, c, true)
	if exists(t, filepath.Join(sc.b.Kept, filepath.FromSlash(store.LocalDir), tmpName)) {
		t.Error("a temporary file of .hedgehug/local was copied")
	}
	if !strings.HasPrefix(sc.b.Kept, c.stores[1].Dir()+keptPrefix) || sc.b.Kept == c.stores[1].Dir()+keptNew {
		t.Fatalf("Kept = %q", sc.b.Kept)
	}
	if exists(t, c.stores[1].Dir()+keptNew) {
		t.Error("the copy in progress is left")
	}
	r, err := gitrepo.Open(sc.b.Kept)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if h, err := r.Head(); err != nil || h.String() != sc.rec.OldHead {
		t.Errorf("Head of the copy = %s, %v; want %s", h, err, sc.rec.OldHead)
	}
	b, err := r.ReadAt(plumbing.NewHash(sc.rec.OldHead), model.TaskPath(sc.gone.ULID))
	if err != nil || !strings.Contains(string(b), "gone by b") {
		t.Errorf("the change that was not put back = %q, %v", b, err)
	}
	if exists(t, filepath.Join(sc.b.Kept, filepath.FromSlash(store.LocalDir), "lock")) {
		t.Error("the lock was copied")
	}
	b2, err := FinishRebuild(c.stores[1])
	if err != nil || b2.Kept != sc.b.Kept {
		t.Fatalf("FinishRebuild = %+v, %v", b2, err)
	}
	if !exists(t, sc.b.Kept) {
		t.Error("the copy was removed")
	}
}

// A removal that fails leaves the store whole, with a warning; the next run
// finishes it.
func TestFinishRebuildRetriesThePrune(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newStoppedScene(t, c, false)
	dir := c.stores[1].Dir()
	unlock := gitrepotest.LockObjects(t, filepath.Join(dir, ".git"))
	b, err := FinishRebuild(c.stores[1])
	unlock()
	if err != nil || b == nil || len(b.Warnings) != 1 {
		t.Fatalf("FinishRebuild = %+v, %v; want one warning", b, err)
	}
	if p := store.RebuildPhaseIn(dir); p != store.RebuildPrune {
		t.Errorf("phase = %q", p)
	}
	if b, err = FinishRebuild(c.stores[1]); err != nil || len(b.Warnings) != 0 {
		t.Fatalf("second FinishRebuild = %+v, %v", b, err)
	}
	checkRebuilt(t, c, sc)
}

// A write that comes during the rebuild waits for the store lock, and lands
// on the new history.
func TestAWriteWaitsForTheRebuild(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	gone := c.addTask(0, "gone")
	keep := c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin(gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	s := c.stores[1]
	// As another process would: a store of its own, opened before the rebuild.
	other, err := store.Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Repo().Close() }()
	held, err := store.LockInOrder(s)
	if err != nil {
		t.Fatal(err)
	}
	rec, b, err := prepare(s, c.opts[1])
	if err != nil || rec == nil {
		t.Fatalf("prepare = %+v, %v", rec, err)
	}
	done := make(chan error)
	go func() {
		_, err := other.Update(c.op(1, "update", keep.ULID), func(tx *store.Tx) error {
			x, err := tx.Task(keep.ULID)
			if err != nil {
				return err
			}
			x.Title = "late"
			return tx.PutTask(x)
		})
		done <- err
	}()
	time.Sleep(50 * time.Millisecond) // the write is waiting, or has not begun
	if err := finish(s, rec, b); err != nil {
		t.Fatal(err)
	}
	held.Release()
	if err := <-done; err != nil {
		t.Fatalf("the write: %v", err)
	}
	c.reopen()
	s = c.stores[1]
	if h, err := s.Repo().Between(mustHead(t, s), plumbing.NewHash(rec.Target)); err != nil || len(h) != 1 {
		t.Fatalf("commits above the target = %+v, %v", h, err)
	}
	if got := c.task(1, keep.ULID); got == nil || got.Title != "late" {
		t.Errorf("keep = %+v", got)
	}
}

// A writer that opened the store before the rebuild is refused while the
// rebuild is stopped in the apply phase: a commit on the old HEAD would stop
// the rebuild from finishing.
func TestAWriteIsRefusedWhileApplyIsStopped(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	// As another process would: a store of its own, opened before the rebuild.
	other, err := store.Open(c.stores[1].Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Repo().Close() }()
	sc := newStoppedScene(t, c, false)
	_, err = other.Update(c.op(1, "update", sc.other.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(sc.other.ULID)
		if err != nil {
			return err
		}
		x.Title = "late"
		return tx.PutTask(x)
	})
	var left *store.LeftoverError
	if !errors.As(err, &left) {
		t.Fatalf("write during the apply phase = %v, want a LeftoverError", err)
	}
	if b, err := FinishRebuild(c.stores[1]); err != nil || b == nil {
		t.Fatalf("FinishRebuild = %+v, %v", b, err)
	}
	checkRebuilt(t, c, sc)
}

// HEAD off main is found before rebuild.json is written, while the store is
// still as it was: nothing is half done.
func TestRebuildStopsBeforeTheRecordWhenHeadIsOffMain(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	gone := c.addTask(0, "gone")
	c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin(gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	s := c.stores[1]
	head := mustHead(t, s)
	if err := os.WriteFile(filepath.Join(s.Dir(), ".git", "HEAD"), []byte(head.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RebuildStore(s, c.opts[1]); !errors.Is(err, gitrepo.ErrWorkTreeChanged) {
		t.Fatalf("RebuildStore = %v, want ErrWorkTreeChanged", err)
	}
	if p := store.RebuildPhaseIn(s.Dir()); p != store.RebuildNone {
		t.Errorf("phase = %q", p)
	}
	c.reopen() // the store still opens
}

// A rebuild that stopped after the caller looked is finished from its
// rebuild.json, and the result is the one of the first.
func TestRebuildStoreFinishesAStoppedRebuild(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newStoppedScene(t, c, false)
	b, err := RebuildStore(c.stores[1], c.opts[1])
	if err != nil || b == nil {
		t.Fatalf("RebuildStore = %+v, %v", b, err)
	}
	checkRebuilt(t, c, sc)
}

// A rebuild finished meanwhile (force-push.json is gone) is not done again.
func TestRebuildStoreDoesNothingWithoutForcePush(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	gone := c.addTask(0, "gone")
	c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin(gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	s := c.stores[1]
	head := mustHead(t, s)
	if err := s.RemoveLocal(forcePushFile); err != nil {
		t.Fatal(err)
	}
	if b, err := RebuildStore(s, c.opts[1]); err != nil || b != nil {
		t.Fatalf("RebuildStore = %+v, %v; want nothing", b, err)
	}
	if got := mustHead(t, s); got != head {
		t.Errorf("HEAD = %s, want %s", got, head)
	}
}
