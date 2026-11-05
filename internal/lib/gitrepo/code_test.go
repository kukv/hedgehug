package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// codeRepo makes a work tree at a new directory with one commit on main.
func codeRepo(t *testing.T) (dir string, first plumbing.Hash) {
	t.Helper()
	dir = t.TempDir()
	r, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir, codeCommit(t, r, dir, "a.txt", "a\n")
}

func codeCommit(t *testing.T, r *git.Repository, dir, name, body string) plumbing.Hash {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add(name); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "t", Email: "t@example.com", When: time.Unix(1_700_000_000, 0)}
	h, err := w.Commit("c "+name, &git.CommitOptions{Author: sig, Committer: sig})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func openCode(t *testing.T, dir string) *Code {
	t.Helper()
	c, err := OpenCode(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestOpenCodeNoRepository(t *testing.T) {
	noGit(t)
	if _, err := OpenCode(t.TempDir()); !errors.Is(err, ErrNoRepository) {
		t.Errorf("OpenCode = %v; want ErrNoRepository", err)
	}
}

func TestOpenCodeFindsRepositoryAbove(t *testing.T) {
	noGit(t)
	dir, _ := codeRepo(t)
	sub := filepath.Join(dir, "x", "y")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	b, ok, err := openCode(t, sub).Branch()
	if err != nil || !ok || b != "main" {
		t.Errorf("Branch = %q, %v, %v", b, ok, err)
	}
}

func TestStartBranchKeepsWorkTreeChanges(t *testing.T) {
	noGit(t)
	dir, first := codeRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := openCode(t, dir)
	if ok, err := c.HasBranch("hh/x"); err != nil || ok {
		t.Fatalf("HasBranch before = %v, %v", ok, err)
	}
	if err := c.StartBranch("hh/x"); err != nil {
		t.Fatal(err)
	}
	if ok, err := c.HasBranch("hh/x"); err != nil || !ok {
		t.Errorf("HasBranch after = %v, %v", ok, err)
	}
	b, ok, err := c.Branch()
	if err != nil || !ok || b != "hh/x" {
		t.Errorf("Branch = %q, %v, %v", b, ok, err)
	}
	r, _ := git.PlainOpen(dir)
	ref, err := r.Reference(plumbing.NewBranchReferenceName("hh/x"), true)
	if err != nil || ref.Hash() != first {
		t.Errorf("hh/x = %v, %v; want %v", ref, err, first)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(got) != "edited\n" {
		t.Errorf("a.txt = %q; the change must stay", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Errorf("new.txt: %v", err)
	}
	st, err := mustWorktree(t, r).Status()
	if err != nil || st.IsClean() {
		t.Errorf("status = %v, %v; want the changes still pending", st, err)
	}
}

func mustWorktree(t *testing.T, r *git.Repository) *git.Worktree {
	t.Helper()
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestStartBranchFromDetachedHead(t *testing.T) {
	noGit(t)
	dir, first := codeRepo(t)
	r, _ := git.PlainOpen(dir)
	if err := r.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, first)); err != nil {
		t.Fatal(err)
	}
	c := openCode(t, dir)
	if b, ok, err := c.Branch(); err != nil || ok || b != "" {
		t.Fatalf("Branch on a detached HEAD = %q, %v, %v", b, ok, err)
	}
	if err := c.StartBranch("hh/d"); err != nil {
		t.Fatal(err)
	}
	if b, ok, _ := c.Branch(); !ok || b != "hh/d" {
		t.Errorf("Branch = %q, %v", b, ok)
	}
}

func TestStartBranchExistingIsAnError(t *testing.T) {
	noGit(t)
	dir, _ := codeRepo(t)
	c := openCode(t, dir)
	if err := c.StartBranch("main"); err == nil {
		t.Error("StartBranch(main) succeeded; it exists")
	}
}

func TestNoCommit(t *testing.T) {
	noGit(t)
	dir := t.TempDir()
	if _, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	}); err != nil {
		t.Fatal(err)
	}
	c := openCode(t, dir)
	if err := c.StartBranch("hh/x"); !errors.Is(err, ErrNoCommit) {
		t.Errorf("StartBranch = %v; want ErrNoCommit", err)
	}
}

func TestBranches(t *testing.T) {
	noGit(t)
	dir, _ := codeRepo(t)
	c := openCode(t, dir)
	for _, n := range []string{"hh/b", "hh/a", "other"} {
		if err := c.StartBranch(n); err != nil {
			t.Fatal(err)
		}
	}
	got, err := c.Branches(func(s string) bool { return len(s) > 3 && s[:3] == "hh/" })
	if err != nil || !slices.Equal(got, []string{"hh/a", "hh/b"}) {
		t.Errorf("Branches = %v, %v", got, err)
	}
}

func TestRemoteURL(t *testing.T) {
	noGit(t)
	dir, _ := codeRepo(t)
	r, _ := git.PlainOpen(dir)
	if _, err := r.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/o/r.git"}}); err != nil {
		t.Fatal(err)
	}
	c := openCode(t, dir)
	if u, ok, err := c.RemoteURL("origin"); err != nil || !ok || u != "https://github.com/o/r.git" {
		t.Errorf("RemoteURL(origin) = %q, %v, %v", u, ok, err)
	}
	if u, ok, err := c.RemoteURL("nope"); err != nil || ok || u != "" {
		t.Errorf("RemoteURL(nope) = %q, %v, %v", u, ok, err)
	}
}

// A linked worktree has a HEAD of its own: starting a branch there leaves the
// main work tree on its branch.
func TestLinkedWorktreeHasOwnHead(t *testing.T) {
	noGit(t)
	dir, first := codeRepo(t)
	wt := linkWorktree(t, dir, "wt", first)
	c := openCode(t, wt)
	if b, ok, err := c.Branch(); err != nil || !ok || b != "wt-branch" {
		t.Fatalf("worktree Branch = %q, %v, %v", b, ok, err)
	}
	if err := c.StartBranch("hh/w"); err != nil {
		t.Fatal(err)
	}
	if b, _, _ := c.Branch(); b != "hh/w" {
		t.Errorf("worktree Branch = %q", b)
	}
	main := openCode(t, dir)
	if b, _, _ := main.Branch(); b != "main" {
		t.Errorf("main work tree Branch = %q; want main", b)
	}
	if ok, _ := main.HasBranch("hh/w"); !ok {
		t.Error("the branch is shared by the repository")
	}
}

// linkWorktree lays out what git worktree add makes, with go-git: a .git file
// in the new directory that points to .git/worktrees/<name>, which has its
// own HEAD on the branch wt-branch at h.
func linkWorktree(t *testing.T, dir, name string, h plumbing.Hash) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), name)
	admin := filepath.Join(dir, ".git", "worktrees", name)
	if err := os.MkdirAll(admin, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wt, 0o750); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(admin, "HEAD"):      "ref: refs/heads/wt-branch\n",
		filepath.Join(admin, "commondir"): "../..\n",
		filepath.Join(admin, "gitdir"):    filepath.Join(wt, ".git") + "\n",
		filepath.Join(wt, ".git"):         "gitdir: " + admin + "\n",
	}
	for p, body := range files {
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r, _ := git.PlainOpen(dir)
	if err := r.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("wt-branch"), h)); err != nil {
		t.Fatal(err)
	}
	return wt
}

func TestPushBranch(t *testing.T) {
	noGit(t)
	remote := newBare(t)
	dir, _ := codeRepo(t)
	r, _ := git.PlainOpen(dir)
	if _, err := r.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}}); err != nil {
		t.Fatal(err)
	}
	c := openCode(t, dir)
	if err := c.StartBranch("hh/p"); err != nil {
		t.Fatal(err)
	}
	r2, _ := git.PlainOpen(dir)
	second := codeCommit(t, r2, dir, "b.txt", "b\n")
	if err := c.PushBranch("origin", "hh/p", Conn{}); err != nil {
		t.Fatal(err)
	}
	if got := bareBranch(t, remote, "hh/p"); got != second {
		t.Errorf("remote hh/p = %v; want %v", got, second)
	}
	// Nothing new to send is a success.
	if err := c.PushBranch("origin", "hh/p", Conn{}); err != nil {
		t.Errorf("second push = %v", err)
	}
}

func bareBranch(t *testing.T, remote, branch string) plumbing.Hash {
	t.Helper()
	r, err := git.PlainOpen(remote)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		t.Fatalf("remote %s: %v", branch, err)
	}
	return ref.Hash()
}

func TestPushBranchNonFastForwardIsRejected(t *testing.T) {
	noGit(t)
	remote := newBare(t)
	dirA, _ := codeRepo(t)
	rA, _ := git.PlainOpen(dirA)
	if _, err := rA.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}}); err != nil {
		t.Fatal(err)
	}
	a := openCode(t, dirA)
	if err := a.StartBranch("hh/n"); err != nil {
		t.Fatal(err)
	}
	codeCommit(t, rA, dirA, "mine.txt", "m\n")
	if err := a.PushBranch("origin", "hh/n", Conn{}); err != nil {
		t.Fatal(err)
	}
	// Another clone with a different history on the same branch name.
	dirB, _ := codeRepo(t)
	rB, _ := git.PlainOpen(dirB)
	if _, err := rB.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}}); err != nil {
		t.Fatal(err)
	}
	b := openCode(t, dirB)
	if err := b.StartBranch("hh/n"); err != nil {
		t.Fatal(err)
	}
	codeCommit(t, rB, dirB, "other.txt", "o\n")
	if err := b.PushBranch("origin", "hh/n", Conn{}); !errors.Is(err, ErrRejected) {
		t.Errorf("PushBranch = %v; want ErrRejected", err)
	}
}

func TestPushBranchDenied(t *testing.T) {
	noGit(t)
	remote := newBare(t)
	dir, _ := codeRepo(t)
	r, _ := git.PlainOpen(dir)
	if _, err := r.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}}); err != nil {
		t.Fatal(err)
	}
	c := openCode(t, dir)
	if err := c.StartBranch("hh/d"); err != nil {
		t.Fatal(err)
	}
	gitrepotest.LockObjects(t, remote)
	if err := c.PushBranch("origin", "hh/d", Conn{}); !errors.Is(err, ErrPushDenied) {
		t.Errorf("PushBranch = %v; want ErrPushDenied", err)
	}
}

func TestPushBranchNoRemote(t *testing.T) {
	noGit(t)
	dir, _ := codeRepo(t)
	c := openCode(t, dir)
	if err := c.PushBranch("origin", "main", Conn{}); err == nil {
		t.Error("PushBranch without a remote succeeded")
	}
}
