package replicate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// hideFile moves the loose blob of the file at rel in clone i out of the
// object store, so that the file cannot be read from HEAD, and opens the
// store again. The blob comes back when the test ends.
func (c *cluster) hideFile(i int, rel string) {
	c.t.Helper()
	dir := c.stores[i].Dir()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		c.t.Fatal(err)
	}
	h := plumbing.ComputeHash(plumbing.BlobObject, data).String()
	loose := filepath.Join(dir, ".git", "objects", h[:2], h[2:])
	if _, err := os.Stat(loose); err != nil {
		c.t.Fatalf("the blob of %s is not a loose object: %v", rel, err)
	}
	aside := filepath.Join(c.t.TempDir(), h)
	if err := os.Rename(loose, aside); err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { _ = os.Rename(aside, loose) })
	// The open handle keeps the objects it has read in memory.
	s, err := store.Open(dir)
	if err != nil {
		c.t.Fatal(err)
	}
	c.stores[i] = s
	// The blob may be in a pack too, which a fetch brings.
	if _, err := s.Repo().ReadFile(c.headTree(i), rel); err == nil {
		c.t.Fatalf("%s can still be read: the blob is packed", rel)
	}
}

// A file of HEAD that cannot be read is not a file that is gone: the repair
// fails and commits nothing.
func TestRepairHeadFailsOnUnreadableFile(t *testing.T) {
	c := newCluster(t, 1)
	a := c.addTask(0, "a")
	b := c.addTask(0, "b")
	c.set(0, b.ULID, func(t *model.Task) { t.ID = a.ID }) // the repair has a commit to make
	before := mustHead(t, c.stores[0])
	c.hideFile(0, model.TaskPath(a.ULID))
	if r, err := RepairHead(c.stores[0], c.opts[0]); err == nil {
		t.Errorf("RepairHead = %+v, nil; want an error", r)
	}
	if after := mustHead(t, c.stores[0]); after != before {
		t.Errorf("HEAD moved to %s from %s", after, before)
	}
}

// The same for a merge: a parent that cannot be read is not a deleted one,
// so nothing is merged and nothing is pushed.
func TestMergeFailsOnUnreadableFile(t *testing.T) {
	c := newCluster(t, 2)
	a := c.addTask(1, "a") // written by clone 1 itself, so that its blob is loose there
	c.sync(1)
	c.sync(0)
	c.addTask(0, "x")
	c.sync(0)
	y := c.addTask(1, "y")
	c.set(1, y.ULID, func(t *model.Task) { p := a.ULID; t.Parent = &p })
	c.hideFile(1, model.TaskPath(a.ULID))
	head, origin := mustHead(t, c.stores[1]), remoteHead(t, c)
	if r, err := Run(c.stores[1], c.opts[1]); err == nil {
		t.Errorf("Run = %+v, nil; want an error", r)
	}
	if after := mustHead(t, c.stores[1]); after != head {
		t.Errorf("HEAD moved to %s from %s", after, head)
	}
	if after := remoteHead(t, c); after != origin {
		t.Errorf("origin moved to %s from %s", after, origin)
	}
}

// remoteHead is the head of the bare remote itself, not of a clone's copy.
func remoteHead(t *testing.T, c *cluster) plumbing.Hash {
	t.Helper()
	r, err := gitrepo.Open(c.remote)
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}
