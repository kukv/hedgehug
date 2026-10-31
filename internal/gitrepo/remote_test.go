package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// noGit empties PATH so that a call to the git binary fails the test.
func noGit(t *testing.T) { t.Setenv("PATH", "") }

func newBare(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "remote.git")
	if err := InitBare(d); err != nil {
		t.Fatal(err)
	}
	return d
}

type clone struct {
	r   *Repo
	dir string
}

// cloneTwo pushes one commit to the bare repository and returns two clones of it.
func cloneTwo(t *testing.T, remote string) (a, b clone) {
	t.Helper()
	dir := t.TempDir()
	seed, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, seed, dir, map[string]string{"seed.md": "s\n"})
	if err := seed.SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	if err := seed.Push("origin"); err != nil {
		t.Fatal(err)
	}
	mk := func() clone {
		d := filepath.Join(t.TempDir(), "clone")
		c, err := Clone(remote, d)
		if err != nil {
			t.Fatal(err)
		}
		return clone{c, d}
	}
	return mk(), mk()
}

func TestPushFetchCloneWithoutGit(t *testing.T) {
	noGit(t)
	remote := newBare(t)
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "a\n"})
	h, _ := r.Head()
	if err := r.SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.RemoteHead("origin"); got != h {
		t.Errorf("RemoteHead after push = %s, want %s", got, h)
	}
	other := filepath.Join(t.TempDir(), "clone")
	c, err := Clone(remote, other)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Head(); got != h {
		t.Errorf("clone HEAD = %s, want %s", got, h)
	}
	if b, err := os.ReadFile(filepath.Join(other, "a.md")); err != nil || string(b) != "a\n" {
		t.Errorf("work tree a.md = %q, %v", b, err)
	}
	commitFiles(t, c, other, map[string]string{"b.md": "b\n"})
	h2, _ := c.Head()
	if err := c.Push("origin"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Fetch("origin")
	if err != nil || got != h2 {
		t.Errorf("Fetch = %s, %v; want %s", got, err, h2)
	}
}

func TestPushRejected(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote) // both clones of one pushed commit
	commitFiles(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	commitFiles(t, b.r, b.dir, map[string]string{"y.md": "b\n"})
	if err := a.r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if err := b.r.Push("origin"); !errors.Is(err, ErrRejected) {
		t.Errorf("second push = %v, want ErrRejected", err)
	}
}

func TestPushNothing(t *testing.T) {
	remote := newBare(t)
	a, _ := cloneTwo(t, remote)
	if err := a.r.Push("origin"); err != nil {
		t.Errorf("push with nothing new = %v", err)
	}
}

func TestFetchEmptyRemote(t *testing.T) {
	remote := newBare(t)
	dir := t.TempDir()
	r, _ := Init(dir)
	if err := r.SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	if h, err := r.Fetch("origin"); err != nil || !h.IsZero() {
		t.Errorf("Fetch of an empty remote = %s, %v", h, err)
	}
}

func TestSetRemotes(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	if err := r.SetRemotes([]string{"/a.git", "/b.git", "/c.git"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemotes([]string{"/a.git", "/c.git"}); err != nil {
		t.Fatal(err)
	}
	if got := r.Remotes(); !slices.Equal(got, []string{"origin", "mirror1"}) {
		t.Errorf("Remotes = %v", got)
	}
	cfg, _ := r.r.Config()
	if u := cfg.Remotes["mirror1"].URLs[0]; u != "/c.git" {
		t.Errorf("mirror1 = %s", u)
	}
	if _, ok := cfg.Remotes["mirror2"]; ok {
		t.Error("mirror2 was not removed")
	}
}

func TestMissingRemote(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	_ = r.SetRemotes([]string{filepath.Join(t.TempDir(), "nope.git")})
	if _, err := r.Fetch("origin"); err == nil {
		t.Error("Fetch of a missing remote succeeded")
	}
}
