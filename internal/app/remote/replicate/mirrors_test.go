package replicate

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// mirrorScene is a cluster of two clones with a mirror, after a rebuild of
// clone 1 on the history origin was rewritten to: the mirror still has the
// old history, with the task gone.
type mirrorScene struct {
	c         *cluster
	mirror    string
	gone      *model.Task
	oldHead   plumbing.Hash // clone 1's HEAD before the rebuild
	rewritten plumbing.Hash // origin's head after the rewrite
	drop      []string      // what the rewrite left out
}

func newMirrorScene(t *testing.T) *mirrorScene {
	t.Helper()
	c := newCluster(t, 2)
	m := &mirrorScene{c: c, mirror: filepath.Join(t.TempDir(), "m.git")}
	if err := gitrepo.InitBare(m.mirror); err != nil {
		t.Fatal(err)
	}
	for i := range c.opts {
		c.opts[i].Remotes = append(c.opts[i].Remotes, m.mirror)
	}
	m.gone = c.addTask(0, "gone secret")
	c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	c.addTask(1, "mine")
	m.oldHead = headOf(t, c, 1)
	m.drop = []string{model.TaskPath(m.gone.ULID), path.Dir(model.CommentPath(m.gone.ULID, model.ULID{})), path.Dir(model.WorklogPath(m.gone.ULID, model.ULID{}))}
	_, m.rewritten = c.rewriteOrigin(m.gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	c.rebuild()
	c.reopen()
	return m
}

func (m *mirrorScene) heldFile() string {
	return filepath.Join(m.c.stores[1].Dir(), filepath.FromSlash(store.LocalDir), mirrorsHeldFile)
}

func hasWarning(r Result, sub string) bool {
	return slices.ContainsFunc(r.Warnings, func(w string) bool { return strings.Contains(w, sub) })
}

// A mirror that was not rewritten still has the purged content: the sync does
// not fetch from it, merge it or push to it, says so, and goes on with origin.
// Once the mirror is rewritten as origin was, it is a mirror again.
func TestRebuildHoldsAMirrorUntilItIsRewritten(t *testing.T) {
	t.Parallel()
	m := newMirrorScene(t)
	c := m.c
	b, err := os.ReadFile(m.heldFile())
	if err != nil || !strings.Contains(string(b), m.mirror) {
		t.Fatalf("mirrors-held.json = %q, %v", b, err)
	}
	for range 2 { // and it stays held
		r := c.sync(1)
		if !slices.Equal(r.Pushed, []string{"origin"}) {
			t.Errorf("pushed %v, want origin only", r.Pushed)
		}
		if !hasWarning(r, "the mirror "+m.mirror+" still has the old history; rewrite it as origin was") {
			t.Errorf("warnings = %q", r.Warnings)
		}
		c.reopen()
		if _, err := c.stores[1].Repo().TreeOf(m.oldHead); !errors.Is(err, plumbing.ErrObjectNotFound) {
			t.Fatalf("the old head reads after the sync: %v", err)
		}
	}
	if has, err := gitrepotest.MainHasFile(m.mirror, model.TaskPath(m.gone.ULID)); err != nil || !has {
		t.Fatalf("the mirror lost the task: %v, %v", has, err)
	}
	// The mirror is rewritten as origin was.
	if _, n := gitrepotest.RewriteRemote(t, m.mirror, m.drop...); n != m.rewritten.String() {
		t.Fatalf("the mirror was rewritten to %s, origin to %s", n, m.rewritten)
	}
	r := c.sync(1)
	if hasWarning(r, "still has the old history") {
		t.Errorf("warnings = %q", r.Warnings)
	}
	if !slices.Equal(r.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("pushed %v, want origin and mirror1", r.Pushed)
	}
	if _, err := os.Stat(m.heldFile()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("mirrors-held.json is left: %v", err)
	}
}

// A mirror the config no longer has is no longer held.
func TestHeldMirrorLeavesWhenTheConfigDropsIt(t *testing.T) {
	t.Parallel()
	m := newMirrorScene(t)
	m.c.opts[1].Remotes = m.c.opts[1].Remotes[:1]
	m.c.sync(1)
	if _, err := os.Stat(m.heldFile()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("mirrors-held.json is left: %v", err)
	}
}

// While the prune of a rebuild keeps failing, the old head is still on the
// disk: a mirror that has it must stay held, or the sync would let it go and
// merge the old history back. Once the rebuild is over it is released.
func TestHeldMirrorStaysHeldWhileThePruneIsLeft(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	mirror := filepath.Join(t.TempDir(), "m.git")
	if err := gitrepo.InitBare(mirror); err != nil {
		t.Fatal(err)
	}
	c.opts[0].Remotes = append(c.opts[0].Remotes, mirror)
	c.addTask(0, "keep")
	c.sync(0)
	c.sync(0) // the mirror's main is now a commit the store has
	s := c.stores[0]
	if err := holdMirrors(s, []string{mirror}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal(store.RebuildFile, []byte(`{"phase":"prune"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		r := c.sync(0)
		if !hasWarning(r, "the mirror "+mirror+" still has the old history") {
			t.Errorf("warnings = %q", r.Warnings)
		}
		if slices.Contains(r.Pushed, "mirror1") {
			t.Errorf("pushed %v to a held mirror", r.Pushed)
		}
		if held, err := readHeld(s); err != nil || !slices.Equal(held, []string{mirror}) {
			t.Fatalf("held = %v, %v", held, err)
		}
	}
	if err := s.RemoveLocal(store.RebuildFile); err != nil {
		t.Fatal(err)
	}
	r := c.sync(0)
	if hasWarning(r, "still has the old history") {
		t.Errorf("warnings = %q", r.Warnings)
	}
	if held, err := readHeld(s); err != nil || len(held) != 0 {
		t.Errorf("held = %v, %v", held, err)
	}
}
