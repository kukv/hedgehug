package usecase_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// worktreeDir makes a repository with origin url and one empty commit, adds a
// git worktree of it on a new branch with the real git, and returns the
// directory of the worktree. The test is skipped without git.
func worktreeDir(t *testing.T, branch, url string) string {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	main := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(gitBin, append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = main
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1") // not the settings of whoever runs the test
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", url)
	run("commit", "-q", "--allow-empty", "-m", "init")
	dir := filepath.Join(t.TempDir(), "wt")
	run("worktree", "add", "-q", "-b", branch, dir)
	return dir
}

// A worktree has a .git file that points into the .git of the main
// repository, where the remotes are. The repo ID comes from there and the
// branch from the HEAD of the worktree.
func TestWorktreeFindsTheRepoAndItsOwnBranch(t *testing.T) {
	t.Parallel()
	w, _ := newWorld(t)
	p := put(t, w.a, "Web release", project("github.com/org-a/web"))
	set := usecasetest.OpenSet(t, w.a, w.b)
	dir := worktreeDir(t, "hh/WEB-1-login", "https://github.com/Org-A/Web.git")
	if fi, err := os.Stat(filepath.Join(dir, ".git")); err != nil || fi.IsDir() {
		t.Fatalf(".git of the worktree = %v, %v; want a file", fi, err)
	}

	// stage 3: the branch of the worktree names WEB-1
	_, cf, _, _, err := resolve(t, set, current.State{}, usecase.Input{Dir: dir, Interactive: true})
	if err != nil || cf == nil || cf.Parent.ULID != w.web1.ULID {
		t.Fatalf("Confirm = %+v, %v; want the task WEB-1 of the branch", cf, err)
	}
	// stage 4 (the branch is declined): the repo ID of origin finds the project
	d := mustDest(t, set, current.State{}, usecase.Input{Dir: dir, Interactive: true, BranchParent: usecase.No})
	if d.Stage != 4 || d.Parent == nil || d.Parent.ULID != p.ULID || d.RepoID != "github.com/org-a/web" {
		t.Errorf("destination = stage %d parent %v repo %q; want stage 4, the project, github.com/org-a/web", d.Stage, d.Parent, d.RepoID)
	}
}
