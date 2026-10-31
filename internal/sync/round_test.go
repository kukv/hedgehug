package sync

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// putFile commits data at rel in clone i.
func (c *cluster) putFile(i int, rel, data string) {
	c.t.Helper()
	if _, err := c.stores[i].Update(c.op(i, "edit", model.ULID{}), func(tx *store.Tx) error {
		return tx.PutFile(rel, []byte(data))
	}); err != nil {
		c.t.Fatal(err)
	}
}

// replace sets *p to v for the test.
func replace[T any](t *testing.T, p *T, v T) {
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

func TestDirtyGitattributes(t *testing.T) {
	c := newCluster(t, 1)
	s := c.stores[0]
	c.putFile(0, ".gitattributes", "tasks/**/*.md merge=hedgehug\n") // setup has lines to add
	p := filepath.Join(s.Dir(), ".gitattributes")
	edit := "tasks/**/*.md merge=hedgehug\n*.png binary\n"
	if err := os.WriteFile(p, []byte(edit), 0o600); err != nil {
		t.Fatal(err)
	}
	head := mustHead(t, s)
	if _, err := Run(s, c.opts[0]); !errors.Is(err, ErrDirty) {
		t.Errorf("Run = %v", err)
	}
	if mustHead(t, s) != head {
		t.Error("HEAD moved")
	}
	if b, _ := os.ReadFile(p); string(b) != edit {
		t.Errorf(".gitattributes = %q", b)
	}
}

// A merge that stops at a dirty file commits nothing, the setup either.
func TestDirtyMergeCommitsNoSetup(t *testing.T) {
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.putFile(1, ".gitattributes", "tasks/**/*.md merge=hedgehug\n")
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.sync(0)
	p := filepath.Join(c.stores[1].Dir(), filepath.FromSlash(model.TaskPath(x.ULID)))
	if err := os.WriteFile(p, []byte("hand edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	head := mustHead(t, c.stores[1])
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) {
		t.Errorf("Run = %v", err)
	}
	if mustHead(t, c.stores[1]) != head {
		t.Error("HEAD moved")
	}
}

// A line with other attributes, or spaced otherwise, counts.
func TestSetupReadsAttributes(t *testing.T) {
	c := newCluster(t, 1)
	c.putFile(0, ".gitattributes", "tasks/**/*.md   diff=md merge=hedgehug\n# *.yaml merge=hedgehug\n")
	c.sync(0)
	b, _ := os.ReadFile(filepath.Join(c.stores[0].Dir(), ".gitattributes"))
	if n := strings.Count(string(b), "tasks/**/*.md"); n != 1 {
		t.Errorf("tasks/**/*.md appears %d times:\n%s", n, b)
	}
	if !strings.HasSuffix(string(b), "\n*.yaml merge=hedgehug\n") {
		t.Errorf("a commented line counted:\n%s", b)
	}
}

func TestCleanStoreSkipsFullRepair(t *testing.T) {
	c := newCluster(t, 1)
	n := 0
	replace(t, &onFullRepair, func() { n++ })
	a := c.addTask(0, "a")
	c.sync(0)
	if n != 0 {
		t.Errorf("a clean store read every file %d times", n)
	}
	b := c.addTask(0, "b")
	c.set(0, b.ULID, func(t *model.Task) { t.ID = a.ID })
	if r := c.sync(0); n != 1 || len(r.Repaired) != 1 {
		t.Errorf("full repairs %d, repaired %+v", n, r.Repaired)
	}
}

func TestHeadMovedCapped(t *testing.T) {
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.set(1, x.ULID, func(t *model.Task) { t.Status = "done" })
	c.sync(0)
	n := 0
	replace(t, &beforeCommit, func() { n++; c.addTask(1, "moves HEAD") })
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, store.ErrHeadMoved) {
		t.Errorf("Run = %v", err)
	}
	if n != tries {
		t.Errorf("computed %d times, want %d", n, tries)
	}
}

func TestMirrorRejectedMergedNextRound(t *testing.T) {
	c := newCluster(t, 2)
	mirror := filepath.Join(t.TempDir(), "m.git")
	if err := gitrepo.InitBare(mirror); err != nil {
		t.Fatal(err)
	}
	c.opts[0].Remotes = append(c.opts[0].Remotes, mirror)
	c.sync(0)
	c.addTask(0, "x")
	var y *model.Task
	c.opts[0].BeforePush = func() {
		if y == nil { // clone 1 pushes to the mirror alone
			y = c.addTask(1, "y")
			c.opts[1].Remotes = []string{mirror}
			c.sync(1)
		}
	}
	r := c.sync(0)
	if !slices.Equal(r.Pushed, []string{"origin", "mirror1"}) || r.Merges != 1 {
		t.Errorf("pushed %v, merges %d", r.Pushed, r.Merges)
	}
	if c.task(0, y.ULID) == nil {
		t.Error("the mirror's task was not merged")
	}
}

func TestPushRoundsRunOut(t *testing.T) {
	replace(t, &retryWait, time.Millisecond)
	c := newCluster(t, 2)
	a := c.addTask(0, "a")
	pushes := 0
	c.opts[0].BeforePush = func() {
		pushes++
		c.addTask(1, "ahead")
		c.sync(1) // origin moves ahead before every push
	}
	_, err := Run(c.stores[0], c.opts[0])
	if !errors.Is(err, gitrepo.ErrRejected) || pushes != rounds {
		t.Errorf("Run = %v after %d pushes", err, pushes)
	}
	if c.task(0, a.ULID) == nil {
		t.Error("the local commit is gone")
	}
}

func TestRepack(t *testing.T) {
	c := newCluster(t, 1)
	replace(t, &repackAt, 0)
	c.addTask(0, "a")
	r := c.sync(0)
	if n, err := c.stores[0].Repo().LooseObjects(); err != nil || n != 0 || len(r.Warnings) != 0 {
		t.Errorf("loose objects %d, %v; warnings %v", n, err, r.Warnings)
	}
}

// Two clones merge the same pair of commits, each from its own side, and
// get the same tree and the same overwritten values.
func TestMergeIsSymmetric(t *testing.T) {
	for _, sameField := range []bool{false, true} {
		c := newCluster(t, 2)
		x := c.addTask(0, "x")
		y := c.addTask(0, "y")
		c.sync(0)
		c.sync(1)
		c.set(0, x.ULID, func(t *model.Task) { t.Title = "from 0"; t.Status = "done" })
		c.set(0, y.ULID, func(t *model.Task) { t.Title = "y0" })
		if sameField {
			c.set(1, x.ULID, func(t *model.Task) { t.Title = "from 1" })
		} else {
			c.set(1, x.ULID, func(t *model.Task) { t.Status = "in_progress" })
		}
		c.addTask(1, "z")
		origin := c.remote
		r2 := filepath.Join(t.TempDir(), "r2.git")
		if err := gitrepo.InitBare(r2); err != nil {
			t.Fatal(err)
		}
		c.sync(0) // origin has clone 0's side
		c.opts[1].Remotes = []string{r2}
		c.sync(1) // r2 has clone 1's side
		c.opts[0].Remotes = []string{r2}
		r0 := c.sync(0) // ours is clone 0's side
		c.opts[1].Remotes = []string{origin}
		r1 := c.sync(1) // ours is clone 1's side
		if r0.Merges != 1 || r1.Merges != 1 {
			t.Fatalf("same=%v: merges %d %d", sameField, r0.Merges, r1.Merges)
		}
		if c.headTree(0) != c.headTree(1) {
			t.Errorf("same=%v: trees differ", sameField)
		}
		key := func(a, b merge.Overwritten) int {
			return strings.Compare(a.Path+a.Field+a.Value, b.Path+b.Field+b.Value)
		}
		slices.SortFunc(r0.Overwritten, key)
		slices.SortFunc(r1.Overwritten, key)
		if !slices.Equal(r0.Overwritten, r1.Overwritten) || len(r0.Overwritten) != 1 { // status or title, both sides changed it
			t.Errorf("same=%v: overwritten %+v and %+v", sameField, r0.Overwritten, r1.Overwritten)
		}
	}
}

func TestTrailerPathWithSpace(t *testing.T) {
	c := newCluster(t, 2)
	const p = "notes/a b.txt"
	c.putFile(0, p, "base\n")
	c.sync(0)
	c.sync(1)
	c.putFile(0, p, "one\n")
	c.putFile(1, p, "two\n")
	c.sync(0)
	c.sync(1)
	e, _ := c.stores[1].Repo().LogFrom(mustHead(t, c.stores[1]), 1)
	v, _ := e[0].Trailer("Hedgehug-Overwritten")
	f := strings.Fields(v)
	if len(f) != 4 || f[0] != url.PathEscape(p) || f[1] != "*" {
		t.Errorf("trailer = %q", v)
	}
	if got, err := url.PathUnescape(f[0]); err != nil || got != p {
		t.Errorf("unescaped = %q, %v", got, err)
	}
}
