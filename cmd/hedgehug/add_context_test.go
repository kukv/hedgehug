package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// cloneDir makes a git repository with origin url, on the branch ("" keeps
// master), and makes it the current directory of the runs.
func (te *testEnv) cloneDir(branch, url string) {
	te.t.Helper()
	dir := te.t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		te.t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{url}}); err != nil {
		te.t.Fatal(err)
	}
	if branch != "" {
		if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(branch))); err != nil {
			te.t.Fatal(err)
		}
	}
	te.dir = dir
}

func TestAddInACloneLinksTheRepoAndGoesUnderItsProject(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.cloneDir("", "https://github.com/Org-A/Web.git")
	p := te.addJSON("Web release", "--kind", "project")
	if got := te.task(p.ULID).Repos; len(got) != 1 || got[0] != "github.com/org-a/web" {
		t.Fatalf("project repos = %q; want the repo ID of origin", got)
	}
	out := te.ok("add", "login page")
	if !strings.HasSuffix(out, "→ added to me / Web release\n") {
		t.Errorf("stdout = %q", out)
	}
	got := te.addJSON("again")
	if got.Store != "me" || got.Parent == nil || *got.Parent != p.ULID || got.Workspace == nil || got.Workspace.Name != "me" {
		t.Errorf("task = %+v; want store me, parent %s, workspace me", got, p.ULID)
	}
	if repos := te.task(got.ULID).Repos; len(repos) != 1 || repos[0] != "github.com/org-a/web" {
		t.Errorf("repos = %q", repos)
	}
	// --no-repo leaves the link off
	n := te.addJSON("unlinked", "--no-repo")
	if repos := te.task(n.ULID).Repos; len(repos) != 0 {
		t.Errorf("--no-repo: repos = %q", repos)
	}
	// an explicit workspace is the root, whatever the repository says
	root := te.addJSON("elsewhere", "--workspace", "me")
	if root.Parent != nil {
		t.Errorf("--workspace: parent = %v; want none (the root)", root.Parent)
	}
	if out := te.ok("add", "root line", "--workspace", "me"); !strings.HasSuffix(out, "→ added to me / me\n") {
		t.Errorf("stdout = %q", out)
	}
}

func TestAddAsksThePlaceOnATerminal(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.cloneDir("", "https://github.com/org-a/web")
	te.addJSON("Web A", "--kind", "project")
	te.addJSON("Web B", "--kind", "project", "--workspace", "me")
	te.terminal = true

	// two projects, none recent: a numbered list and no default
	te.stdin = "2\n"
	code, out, errs := te.run("add", "x")
	if code != 0 || !strings.HasSuffix(out, "→ added to me / Web B\n") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if !strings.Contains(errs, "  1) me:ME-1 Web A\n  2) me:ME-2 Web B\nProject: ") {
		t.Errorf("stderr = %q", errs)
	}
	// the choice is recent now: Enter takes it
	te.stdin = "\n"
	code, out, errs = te.run("add", "y")
	if code != 0 || !strings.HasSuffix(out, "→ added to me / Web B\n") || !strings.Contains(errs, "Project [2]: ") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	te.stdin = "1\n"
	if out := te.ok("add", "z"); !strings.HasSuffix(out, "→ added to me / Web A\n") {
		t.Errorf("stdout = %q", out)
	}
	te.stdin = "3\n"
	if code, _, _ := te.run("add", "w"); code != 2 {
		t.Errorf("a number out of range: exit %d; want 2", code)
	}

	// not a terminal: recent (Web A, from the last pick) is used
	te.terminal = false
	te.stdin = ""
	if out := te.ok("add", "q"); !strings.HasSuffix(out, "→ added to me / Web A\n") {
		t.Errorf("not a terminal, with recent: stdout %q", out)
	}
}

func TestAddWithoutRecentAndNoTerminalNamesTheCandidates(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.cloneDir("", "https://github.com/org-a/web")
	te.addJSON("Web A", "--kind", "project")
	te.addJSON("Web B", "--kind", "project", "--workspace", "me")
	code, _, errs := te.run("add", "x")
	if code != 2 || !strings.Contains(errs, "me:ME-1") || !strings.Contains(errs, "me:ME-2") {
		t.Errorf("exit %d, stderr %q; want 2 with the candidates", code, errs)
	}
}

func TestAddAsksAboutTheBranch(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("Login work")
	te.cloneDir("hh/"+p.ID+"-login", "https://github.com/org-a/web")
	te.terminal = true
	te.stdin = "\n"
	code, out, errs := te.run("add", "child")
	if code != 0 || !strings.Contains(errs, "Add as a child of "+p.ID+" Login work? [Y/n] ") || !strings.HasSuffix(out, "→ added to me / Login work\n") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	te.stdin = "n\n"
	code, out, _ = te.run("add", "root")
	if code != 0 || !strings.HasSuffix(out, "→ added to me / me\n") {
		t.Errorf("n: exit %d, stdout %q", code, out)
	}
	// not a terminal: the branch is not asked about
	te.terminal = false
	if out := te.ok("add", "quiet"); !strings.HasSuffix(out, "→ added to me / me\n") {
		t.Errorf("not a terminal: stdout %q", out)
	}
}

func TestAddAsksTheStoreAndTheWorkspace(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("store", "init", "other", "--prefix", "OT")
	te.dropDefaultStore()
	te.terminal = true
	te.stdin = "2\n"
	code, out, errs := te.run("add", "x")
	if code != 0 || !strings.HasSuffix(out, "→ added to other / other\n") || !strings.Contains(errs, "  1) me\n  2) other\nStore: ") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	te.terminal = false
	te.stdin = ""
	code, _, errs = te.run("add", "y")
	if code != 2 || !strings.Contains(errs, "--store") || !strings.Contains(errs, "hedgehug store default") {
		t.Errorf("exit %d, stderr %q", code, errs)
	}
	te.ok("workspace", "add", "second", "--prefix", "SC", "--store", "other")
	code, _, errs = te.run("add", "z", "--store", "other")
	if code != 2 || !strings.Contains(errs, "--workspace") {
		t.Errorf("several workspaces: exit %d, stderr %q", code, errs)
	}
	te.terminal = true
	te.stdin = "2\n"
	if out := te.ok("add", "w", "--store", "other"); !strings.HasSuffix(out, "→ added to other / second\n") {
		t.Errorf("stdout %q", out)
	}
}

// dropDefaultStore takes default_store out of config.yaml (the first store
// made is the default).
func (te *testEnv) dropDefaultStore() {
	te.t.Helper()
	p := filepath.Join(te.hhHome, "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		te.t.Fatal(err)
	}
	var keep []string
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(l, "default_store:") {
			keep = append(keep, l)
		}
	}
	if err := os.WriteFile(p, []byte(strings.Join(keep, "\n")), 0o600); err != nil {
		te.t.Fatal(err)
	}
}
