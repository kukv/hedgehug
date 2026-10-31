package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/config"
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
	if err := a.r.r.Storer.RemoveReference(trackingRef("origin")); err != nil {
		t.Fatal(err)
	}
	if err := a.r.Push("origin"); err != nil {
		t.Errorf("push with nothing new = %v", err)
	}
	h, _ := a.r.Head()
	if got, _ := a.r.RemoteHead("origin"); got != h {
		t.Errorf("RemoteHead = %s, want %s", got, h)
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
	if _, err := r.r.CreateRemote(&config.RemoteConfig{Name: "upstream", URLs: []string{"/u.git"}}); err != nil {
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
	if _, ok := cfg.Remotes["upstream"]; !ok {
		t.Error("upstream was removed")
	}
}

func TestLocalPath(t *testing.T) {
	for in, want := range map[string]string{
		"/C:/x/y.git":    "C:/x/y.git",
		`/d:\x\y.git`:    `d:\x\y.git`,
		"/home/u/y.git":  "/home/u/y.git",
		"relative/y.git": "relative/y.git",
	} {
		if got := localPath(in); got != want {
			t.Errorf("localPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCloneFailureLeavesNoDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "clone")
	if _, err := Clone(filepath.Join(t.TempDir(), "nope.git"), dir); err == nil {
		t.Fatal("Clone of a missing remote succeeded")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("directory left behind: %v", err)
	}
}

func TestMissingRemote(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	if err := r.SetRemotes([]string{filepath.Join(t.TempDir(), "nope.git")}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Fetch("origin"); err == nil {
		t.Error("Fetch of a missing remote succeeded")
	}
}

func TestCloneFailureInExistingDirectoryKeepsIt(t *testing.T) {
	dir := t.TempDir()
	if _, err := Clone(filepath.Join(t.TempDir(), "nope.git"), dir); err == nil {
		t.Fatal("Clone of a missing remote succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Errorf("directory = %v, %v; want it kept and empty", entries, err)
	}
}

func TestCloneCleansUpWhenTheLastStepFails(t *testing.T) {
	remote := newBare(t)
	seed := t.TempDir()
	s, _ := Init(seed)
	commitFiles(t, s, seed, map[string]string{"a.md": "a\n"})
	_ = s.SetRemotes([]string{remote})
	if err := s.Push("origin"); err != nil {
		t.Fatal(err)
	}
	orig := afterClone
	afterClone = func(*Repo, string) error { return errors.New("boom") }
	t.Cleanup(func() { afterClone = orig })

	fresh := filepath.Join(t.TempDir(), "clone")
	if _, err := Clone(remote, fresh); err == nil {
		t.Fatal("Clone succeeded")
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Errorf("created directory left behind: %v", err)
	}
	existing := t.TempDir()
	if _, err := Clone(remote, existing); err == nil {
		t.Fatal("Clone succeeded")
	}
	if entries, err := os.ReadDir(existing); err != nil || len(entries) != 0 {
		t.Errorf("existing directory = %v, %v; want kept and empty", entries, err)
	}
}

func TestCloneRefusesNonEmptyDirectory(t *testing.T) {
	remote := newBare(t)
	dir := t.TempDir()
	writeFile(t, dir, "mine.txt", "keep\n")
	if _, err := Clone(remote, dir); err == nil {
		t.Fatal("Clone into a non-empty directory succeeded")
	}
	if b, err := os.ReadFile(filepath.Join(dir, "mine.txt")); err != nil || string(b) != "keep\n" {
		t.Errorf("user file = %q, %v", b, err)
	}
}
