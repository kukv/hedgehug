package sync

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// rewriteLocally makes the first clone drop the task u from its whole history, as
// purge --rewrite-history does, and commits the record. before is the
// record's before; the zero hash means the head the clone had. It pushes
// nothing. It returns the old head and the record.
func (c *cluster) rewriteLocally(u model.ULID, before plumbing.Hash, record bool) (plumbing.Hash, model.Rewrite) {
	c.t.Helper()
	s := c.stores[0]
	r := s.Repo()
	old, err := r.Head()
	if err != nil {
		c.t.Fatal(err)
	}
	if before.IsZero() {
		before = old
	}
	nh, _, err := r.RewriteHistory(old, gitrepo.Rewrite{Drop: func(p string) bool { return strings.Contains(p, u.String()) }})
	if err != nil {
		c.t.Fatal(err)
	}
	if err := r.ResetMain(nh); err != nil {
		c.t.Fatal(err)
	}
	rec := model.Rewrite{At: time.Now().UTC().Truncate(time.Second), Before: before.String(), ULIDs: []model.ULID{u}}
	if record {
		if _, err := s.Update(c.op(0, "rewrite-history", model.ULID{}), func(tx *store.Tx) error {
			data, err := model.EncodeRewrites([]model.Rewrite{rec})
			if err != nil {
				return err
			}
			return tx.PutFile(model.RewritesPath, data)
		}); err != nil {
			c.t.Fatal(err)
		}
	}
	return old, rec
}

func (c *cluster) forcePush() {
	c.t.Helper()
	if err := c.stores[0].Repo().ForcePush("origin", plumbing.ZeroHash); err != nil {
		c.t.Fatal(err)
	}
}

func headOf(t *testing.T, c *cluster, i int) plumbing.Hash {
	t.Helper()
	h, err := c.stores[i].Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func trackOf(t *testing.T, c *cluster) plumbing.Hash {
	t.Helper()
	h, err := c.stores[1].Repo().RemoteHead("origin")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// A rewrite with a record is found by the other clone, which changes
// nothing.
func TestRunFindsRecordedRewrite(t *testing.T) {
	c := newCluster(t, 2)
	keep := c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	headB, trackB := headOf(t, c, 1), trackOf(t, c)

	_, rec := c.rewriteLocally(gone.ULID, plumbing.ZeroHash, true)
	c.forcePush()

	res := c.sync(1)
	if res.Rewrite == nil || !res.Rewrite.At.Equal(rec.At) || res.Rewrite.Before != rec.Before {
		t.Fatalf("Result.Rewrite = %+v, want %+v", res.Rewrite, rec)
	}
	if got := headOf(t, c, 1); got != headB {
		t.Errorf("HEAD moved to %s", got)
	}
	if got := trackOf(t, c); got != trackB {
		t.Errorf("tracking = %s, want %s", got, trackB)
	}
	if c.task(1, gone.ULID) == nil || c.task(1, keep.ULID) == nil {
		t.Error("the clone changed")
	}
	if _, ok, err := ReadForcePush(c.stores[1]); err != nil || ok {
		t.Errorf("force-push.json = %v, %v", ok, err)
	}
	// A record that was taken in is not a rewrite any more.
	if err := WriteApplied(c.stores[1], []time.Time{rec.At}); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Errorf("Run with the record applied = %v, want ErrForcePush", err)
	}
}

// A force push without a record stops the sync, leaves the clone as it was
// and writes force-push.json; the file goes when origin is as it was.
func TestRunUnexpectedForcePush(t *testing.T) {
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	headB, trackB := headOf(t, c, 1), trackOf(t, c)

	old, _ := c.rewriteLocally(gone.ULID, plumbing.ZeroHash, false)
	c.forcePush()
	newHead := headOf(t, c, 0)

	_, err := Run(c.stores[1], c.opts[1])
	if !errors.Is(err, ErrForcePush) {
		t.Fatalf("Run = %v, want ErrForcePush", err)
	}
	if got := headOf(t, c, 1); got != headB {
		t.Errorf("HEAD moved to %s", got)
	}
	if got := trackOf(t, c); got != trackB {
		t.Errorf("tracking = %s, want %s", got, trackB)
	}
	f, ok, err := ReadForcePush(c.stores[1])
	if err != nil || !ok || f.Origin != newHead.String() || f.Tracked != trackB.String() || f.Checked.IsZero() {
		t.Errorf("force-push.json = %+v, %v, %v", f, ok, err)
	}

	// origin goes back: the next sync is quiet and the file goes.
	if err := c.stores[0].Repo().ResetMain(old); err != nil {
		t.Fatal(err)
	}
	c.forcePush()
	c.sync(1)
	if _, ok, err := ReadForcePush(c.stores[1]); err != nil || ok {
		t.Errorf("force-push.json after origin went back = %v, %v", ok, err)
	}
}

// A record whose before is not what the clone followed is no record.
func TestRunRecordWithForeignBefore(t *testing.T) {
	c := newCluster(t, 2)
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	headB := headOf(t, c, 1)
	cm, err := c.stores[0].Repo().Between(headB, plumbing.ZeroHash)
	if err != nil || len(cm) < 2 {
		t.Fatalf("history = %d commits, %v", len(cm), err)
	}
	parent := cm[len(cm)-2].Hash // an ancestor of what B followed, not a descendant

	c.rewriteLocally(gone.ULID, parent, true)
	c.forcePush()
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("Run = %v, want ErrForcePush", err)
	}
}

// A mirror that still has the history before a rewrite is not merged: it is
// force pushed to, and what the rewrite removed does not come back.
func TestRunMirrorWithOldHistory(t *testing.T) {
	c := newCluster(t, 1)
	mirror := filepath.Join(t.TempDir(), "m.git")
	if err := gitrepo.InitBare(mirror); err != nil {
		t.Fatal(err)
	}
	c.opts[0].Remotes = append(c.opts[0].Remotes, mirror)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)

	c.rewriteLocally(gone.ULID, plumbing.ZeroHash, true)
	c.forcePush() // the mirror keeps the old history
	newHead := headOf(t, c, 0)

	res := c.sync(0)
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %v", res.Warnings)
	}
	if res.Merges != 0 || !slices.Equal(res.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("merges %d, pushed %v; want 0 and origin, mirror1", res.Merges, res.Pushed)
	}
	if c.task(0, gone.ULID) != nil {
		t.Error("the removed task came back")
	}
	if got := headOf(t, c, 0); got != newHead {
		t.Errorf("HEAD = %s, want %s (nothing merged)", got, newHead)
	}
	if got, err := c.stores[0].Repo().RemoteHead("mirror1"); err != nil || got != newHead {
		t.Errorf("mirror = %s, %v, want %s", got, err, newHead)
	}
}

// A mirror that lags behind the rewrite (its head is an ancestor of before,
// and holds the purged task) is force pushed to, not merged: the merge
// would bring the purged task back.
func TestRunMirrorBehindBefore(t *testing.T) {
	c := newCluster(t, 1)
	mirror := filepath.Join(t.TempDir(), "m.git")
	if err := gitrepo.InitBare(mirror); err != nil {
		t.Fatal(err)
	}
	origin := c.opts[0].Remotes
	c.opts[0].Remotes = append(slices.Clone(origin), mirror)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0) // the mirror stops at the commit that adds gone
	mirrorHead := headOf(t, c, 0)

	c.opts[0].Remotes = origin
	c.addTask(0, "later")
	c.sync(0)
	c.rewriteLocally(gone.ULID, plumbing.ZeroHash, true)
	c.forcePush()
	newHead := headOf(t, c, 0)
	c.opts[0].Remotes = append(slices.Clone(origin), mirror)

	res := c.sync(0)
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %v", res.Warnings)
	}
	if res.Merges != 0 || !slices.Equal(res.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("merges %d, pushed %v; want 0 and origin, mirror1", res.Merges, res.Pushed)
	}
	if c.task(0, gone.ULID) != nil {
		t.Error("the purged task came back")
	}
	if got := headOf(t, c, 0); got != newHead {
		t.Errorf("HEAD = %s, want %s (nothing merged)", got, newHead)
	}
	if got, err := c.stores[0].Repo().RemoteHead("mirror1"); err != nil || got != newHead || got == mirrorHead {
		t.Errorf("mirror = %s, %v, want %s", got, err, newHead)
	}
}

// A mirror with commits on top of before has commits no history of origin
// has: it is neither merged nor force pushed to, and a warning tells so.
func TestRunMirrorWithCommitsOfNeitherHistory(t *testing.T) {
	c := newCluster(t, 1)
	mirror := filepath.Join(t.TempDir(), "m.git")
	if err := gitrepo.InitBare(mirror); err != nil {
		t.Fatal(err)
	}
	c.opts[0].Remotes = append(c.opts[0].Remotes, mirror)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	before := headOf(t, c, 0)
	c.addTask(0, "extra")
	c.sync(0) // the mirror holds before and a commit on top of it
	mirrorHead := headOf(t, c, 0)

	c.rewriteLocally(gone.ULID, before, true)
	c.forcePush()
	newHead := headOf(t, c, 0)

	res := c.sync(0)
	want := "mirror1 has commits that are in neither history; it is not pushed"
	if !slices.Contains(res.Warnings, want) {
		t.Errorf("warnings = %v, want %q", res.Warnings, want)
	}
	if res.Merges != 0 || !slices.Equal(res.Pushed, []string{"origin"}) {
		t.Errorf("merges %d, pushed %v; want 0 and origin", res.Merges, res.Pushed)
	}
	if got := headOf(t, c, 0); got != newHead {
		t.Errorf("HEAD = %s, want %s (nothing merged)", got, newHead)
	}
	if got, err := c.stores[0].Repo().RemoteHead("mirror1"); err != nil || got != mirrorHead {
		t.Errorf("mirror = %s, %v, want it untouched at %s", got, err, mirrorHead)
	}
}

// A mirror fetched for the first time that holds a before is force pushed
// to, not merged.
func TestRunMirrorFirstFetchHoldsBefore(t *testing.T) {
	c := newCluster(t, 1)
	mirror := filepath.Join(t.TempDir(), "m.git")
	if err := gitrepo.InitBare(mirror); err != nil {
		t.Fatal(err)
	}
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	// Someone else fills the mirror with the history as it is now.
	other, err := gitrepo.Clone(c.remote, filepath.Join(t.TempDir(), "o"))
	if err != nil {
		t.Fatal(err)
	}
	if err := other.SetRemotes([]string{c.remote, mirror}); err != nil {
		t.Fatal(err)
	}
	if err := other.Push("mirror1"); err != nil {
		t.Fatal(err)
	}

	c.rewriteLocally(gone.ULID, plumbing.ZeroHash, true)
	c.forcePush()
	newHead := headOf(t, c, 0)
	c.opts[0].Remotes = append(c.opts[0].Remotes, mirror)
	if got, _ := c.stores[0].Repo().RemoteHead("mirror1"); !got.IsZero() {
		t.Fatalf("the mirror was fetched already: %s", got)
	}

	res := c.sync(0)
	if res.Merges != 0 || !slices.Equal(res.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("merges %d, pushed %v; want 0 and origin, mirror1", res.Merges, res.Pushed)
	}
	if c.task(0, gone.ULID) != nil || headOf(t, c, 0) != newHead {
		t.Error("the removed task came back")
	}
	if got, err := c.stores[0].Repo().RemoteHead("mirror1"); err != nil || got != newHead {
		t.Errorf("mirror = %s, %v, want %s", got, err, newHead)
	}
}

// A clone that lags behind (the rewriter pushed a commit it never fetched,
// then rewrote) finds the record by the commit it followed (D33).
func TestRunFindsRewriteOfALaggingClone(t *testing.T) {
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	headB := headOf(t, c, 1)

	c.addTask(0, "extra")
	c.sync(0) // the commit clone 1 never fetches
	_, rec := c.rewriteLocally(gone.ULID, plumbing.ZeroHash, true)
	c.forcePush()

	res := c.sync(1)
	if res.Rewrite == nil || !res.Rewrite.At.Equal(rec.At) {
		t.Fatalf("Result.Rewrite = %+v, want %+v", res.Rewrite, rec)
	}
	if got := headOf(t, c, 1); got != headB {
		t.Errorf("HEAD moved to %s", got)
	}
}

// A record whose before the clone does not have, on a history that holds
// nothing the clone followed, is an unexpected force push.
func TestRunUnrelatedForcePushWithARecord(t *testing.T) {
	c := newCluster(t, 2)
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	c.addTask(1, "b's")
	c.sync(1) // origin and clone 1 have a commit that clone 0 does not

	c.rewriteLocally(gone.ULID, plumbing.NewHash("1234567890123456789012345678901234567890"), true)
	c.forcePush()
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("Run = %v, want ErrForcePush", err)
	}
}
