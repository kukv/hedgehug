package cli

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

func (te *testEnv) withIssues() *fakegh.Server {
	te.t.Helper()
	fake := te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}, Labels: []string{"hh"}}},
	})
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.env["HH_TOKEN"] = "t-alice"
	te.ok("store", "init", "me", "--prefix", "ME", "--account", "me")
	return fake
}

// fails runs a command that ends in code, and returns what it said.
func (te *testEnv) fails(code int, args ...string) (stdout, stderr string) {
	te.t.Helper()
	got, stdout, stderr := te.run(args...)
	if got != code {
		te.t.Errorf("%v: code %d, want %d\n%s", args, got, code, stderr)
	}
	return stdout, stderr
}

func TestIssueCommand(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	fake := te.withIssues()
	te.ok("add", "Fix the login")
	if out := te.ok("issue", "ME-1", "--repo", "org-a/web"); !strings.HasPrefix(out, "Opened org-a/web#1 for ME-1 ") {
		t.Errorf("issue:\n%s", out)
	}
	r, _ := fake.Repo("org-a/web")
	if len(r.Issues) != 1 || r.Issues[0].Title != "Fix the login" {
		t.Errorf("issues %+v", r.Issues)
	}
	if out := te.ok("show", "ME-1"); !strings.Contains(out, "org-a/web#1") {
		t.Errorf("show:\n%s", out)
	}
	te.ok("add", "Other")
	var w integration.IssueWrite
	strictly(t, te.ok("issue", "ME-2", "--repo", "org-a/web", "--json"), &w)
	if w.Ref != "github.com/org-a/web/issues/2" || w.Warnings == nil || w.Task == nil {
		t.Errorf("json %+v", w)
	}
}

func TestIssueCommandErrors(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.withIssues()
	te.ok("add", "a")
	if _, errOut := te.fails(2, "issue"); !strings.Contains(errOut, "accepts 1 arg") && errOut == "" {
		t.Errorf("no argument: %s", errOut)
	}
	if _, errOut := te.fails(2, "issue", "ME-1", "--repo", "nope"); !strings.Contains(errOut, "want owner/repo") {
		t.Errorf("bad repo: %s", errOut)
	}
}

func TestGHImportCommand(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	fake := te.withIssues()
	te.ok("add", "already here")
	for _, title := range []string{"A", "B"} {
		n, err := fake.OpenIssue("org-a/web", title, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := fake.TagIssue("org-a/web", n, nil, []string{"hh"}, ""); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"gh", "import", "org-a/web", "--label", "hh"}
	if out := te.ok(append(args, "--dry-run")...); out != "Would import 2 issues of github.com/org-a/web (0 linked already)\n" {
		t.Errorf("dry run: %q", out)
	}
	if out := te.ok(args...); out != "Imported 2 issues of github.com/org-a/web (0 linked already): ME-2, ME-3\n" {
		t.Errorf("import: %q", out)
	}
	if out := te.ok(args...); out != "Nothing to import from github.com/org-a/web (2 linked already)\n" {
		t.Errorf("again: %q", out)
	}
	var r integration.ImportResult
	strictly(t, te.ok(append(args, "--json")...), &r)
	if r.Skipped != 2 || r.Created == nil || r.Warnings == nil {
		t.Errorf("json %+v", r)
	}
	if _, errOut := te.fails(2, "gh", "import", "org-a/web"); !strings.Contains(errOut, "label") {
		t.Errorf("no label: %s", errOut)
	}
	if _, errOut := te.fails(2, "gh"); !strings.Contains(errOut, "a subcommand is needed") {
		t.Errorf("no subcommand: %s", errOut)
	}
}
