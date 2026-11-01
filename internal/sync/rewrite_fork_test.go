package sync

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

// forkedMirror makes a cluster of two clones and a mirror that clone 1 filled
// with a task of its own on top of the commit both share (the mirror never
// got the rewrite). Clone 0 then records a rewrite of the task purges(own)
// names and force pushes it, and gets the mirror among its remotes. It
// returns the cluster, the task in the mirror and the mirror's head.
func forkedMirror(t *testing.T, purges func(own *model.Task) model.ULID) (*cluster, *model.Task, plumbing.Hash) {
	t.Helper()
	c := newCluster(t, 2)
	mirror := filepath.Join(t.TempDir(), "m.git")
	if err := gitrepo.InitBare(mirror); err != nil {
		t.Fatal(err)
	}
	c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	own := c.addTask(1, "own to the mirror")
	r1 := c.stores[1].Repo()
	if err := r1.SetRemotes([]string{c.remote, mirror}); err != nil {
		t.Fatal(err)
	}
	if err := r1.Push("mirror1"); err != nil {
		t.Fatal(err)
	}
	mirrorHead := headOf(t, c, 1)
	c.rewriteLocally(purges(own), plumbing.ZeroHash, true)
	c.forcePush()
	c.opts[0].Remotes = append(c.opts[0].Remotes, mirror)
	return c, own, mirrorHead
}

// A mirror that branched off before the rewrite and has a purged task in a
// commit of its own is neither merged (the task would come back) nor force
// pushed to (its commits would be lost): it is warned about.
func TestRunMirrorForkedBeforeBeforeHoldsPurgedTask(t *testing.T) {
	t.Parallel()
	c, own, mirrorHead := forkedMirror(t, func(own *model.Task) model.ULID { return own.ULID })
	newHead := headOf(t, c, 0)
	res := c.sync(0)
	want := "mirror1 has commits that are in neither history; it is not pushed"
	if !slices.Contains(res.Warnings, want) {
		t.Errorf("warnings = %v, want %q", res.Warnings, want)
	}
	if res.Merges != 0 || !slices.Equal(res.Pushed, []string{"origin"}) {
		t.Errorf("merges %d, pushed %v; want 0 and origin", res.Merges, res.Pushed)
	}
	if c.task(0, own.ULID) != nil || headOf(t, c, 0) != newHead {
		t.Error("the purged task came back")
	}
	if got, err := c.stores[0].Repo().RemoteHead("mirror1"); err != nil || got != mirrorHead {
		t.Errorf("mirror = %s, %v, want it untouched at %s", got, err, mirrorHead)
	}
}

// The same mirror, when its own commits touch nothing a rewrite purged, is
// merged as usual.
func TestRunMirrorForkedBeforeBeforeWithOtherWorkIsMerged(t *testing.T) {
	t.Parallel()
	c, own, _ := forkedMirror(t, func(*model.Task) model.ULID { return model.NewULID() })
	res := c.sync(0)
	if len(res.Warnings) != 0 || res.Merges != 1 || !slices.Equal(res.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("warnings %v, merges %d, pushed %v; want a merge pushed to both", res.Warnings, res.Merges, res.Pushed)
	}
	if c.task(0, own.ULID) == nil {
		t.Error("the mirror's task was not merged")
	}
}
