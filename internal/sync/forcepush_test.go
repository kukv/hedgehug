package sync

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// rewriteOrigin makes origin a history of its own, as a git filter-repo
// leaves one, without the files of the tasks in drop (the task, its comments
// and its worklog). The clones that followed the old history find that
// origin is not a descendant of it. It returns the old head and the new one.
func (c *cluster) rewriteOrigin(drop ...model.ULID) (old, rewritten plumbing.Hash) {
	c.t.Helper()
	var paths []string
	for _, u := range drop {
		paths = append(paths, model.TaskPath(u), path.Dir(model.CommentPath(u, model.ULID{})), path.Dir(model.WorklogPath(u, model.ULID{})))
	}
	o, n := gitrepotest.RewriteRemote(c.t, c.remote, paths...)
	return plumbing.NewHash(o), plumbing.NewHash(n)
}

// cloneFresh adds a clone made from origin as it is now, and returns its
// index.
func (c *cluster) cloneFresh() int {
	c.t.Helper()
	i := len(c.stores)
	dir := filepath.Join(c.t.TempDir(), fmt.Sprint("s", i))
	if _, err := gitrepo.Clone(c.remote, dir); err != nil {
		c.t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		c.t.Fatal(err)
	}
	c.add(s, i)
	return i
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

// A force push stops the sync, leaves the clone as it was and writes
// force-push.json; the file goes when origin follows the clone again.
func TestRunStopsOnAForcePush(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	headB, trackB := headOf(t, c, 1), trackOf(t, c)

	old, newHead := c.rewriteOrigin(gone.ULID)
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
	if c.task(1, gone.ULID) == nil {
		t.Error("the clone changed")
	}

	// origin goes back: the next sync is quiet and the file goes.
	gitrepotest.SetRemoteMain(t, c.remote, old.String())
	c.sync(1)
	if _, ok, err := ReadForcePush(c.stores[1]); err != nil || ok {
		t.Errorf("force-push.json after origin went back = %v, %v", ok, err)
	}
}

// A history of origin with no task dropped, only other commits, is a force
// push too: there is no record to tell it from one that removed a task.
func TestRunStopsOnARewriteThatDropsNothing(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin()
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("Run = %v, want ErrForcePush", err)
	}
}
