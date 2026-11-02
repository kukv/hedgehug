package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// worktreeDir makes a repository with origin url and one empty commit, adds a
// worktree of it with the real git, and makes the worktree the current
// directory of the runs. The test is skipped without git.
func (te *testEnv) worktreeDir(branch, url string) {
	te.t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		te.t.Skip("git is not installed")
	}
	main := te.t.TempDir()
	run := func(args ...string) {
		te.t.Helper()
		cmd := exec.Command(gitBin, append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = main
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1") // not the settings of whoever runs the test
		if out, err := cmd.CombinedOutput(); err != nil {
			te.t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", url)
	run("commit", "-q", "--allow-empty", "-m", "init")
	dir := filepath.Join(te.t.TempDir(), "wt")
	run("worktree", "add", "-q", "-b", branch, dir)
	te.dir = dir
}

func TestAddInAWorktreeFindsTheRepo(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.worktreeDir("feature", "https://github.com/Org-A/Web.git")
	p := te.addJSON("Web release", "--kind", "project", "--repo")
	if got := te.task(p.ULID).Repos; len(got) != 1 || got[0] != "github.com/org-a/web" {
		t.Fatalf("project repos = %q; want the repo ID of origin, read through the worktree", got)
	}
	out := te.ok("add", "from the worktree")
	if !strings.HasSuffix(out, "→ added to me / Web release\n") {
		t.Errorf("stdout = %q; want the project found by the repo ID", out)
	}
}
