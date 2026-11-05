package cli

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// A link written by hand in other letters gets its state from sync, is not
// doubled by hh link, and shows in hh show.
func TestSyncTakesALinkWrittenInOtherLetters(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	fake := te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.env["HH_TOKEN"] = "t-alice"
	te.ok("store", "init", "me", "--prefix", "ME", "--account", "me")
	a := te.addJSON("a")
	te.ok("link", "ME-1", "--pr", "org-a/web#1")
	te.handEdit("me", "ME-1", "github.com/org-a/web/pull/1", "github.com/Org-A/web/pull/1")
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	te.ok("sync", "--store", "me")
	if out := te.ok("show", "ME-1"); !strings.Contains(out, "pull request  org-a/web#1  open  review: none") {
		t.Errorf("show:\n%s", out)
	}
	if code, _, _ := te.run("link", "ME-1", "--pr", "org-a/web#1"); code == 0 {
		t.Error("linking again succeeded")
	}
	if n := len(te.task(a.ULID).Links); n != 1 {
		t.Errorf("links = %d", n)
	}
}

// TestSyncBringsGitHubIntoTheTask follows the 5a completion condition through
// the commands: a pull request on a branch named after the task is found by
// sync, shown on the task, matched by has:pr, and gone from both after unlink.
func TestSyncBringsGitHubIntoTheTask(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	fake := te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.env["HH_TOKEN"] = "t-alice"
	te.ok("store", "init", "me", "--prefix", "ME", "--account", "me")
	te.ok("add", "a", "-p", "repos=github.com/org-a/web")
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	te.ok("sync", "--store", "me")
	if out := te.ok("show", "ME-1"); !strings.Contains(out, "pull request  org-a/web#1  open  review: none") {
		t.Errorf("show:\n%s", out)
	}
	if out := te.ok("ls", "has:pr"); !strings.Contains(out, "ME-1") {
		t.Errorf("has:pr:\n%s", out)
	}
	te.ok("unlink", "ME-1", "--pr", "org-a/web#1")
	if out := te.ok("ls", "has:pr"); strings.Contains(out, "ME-1") {
		t.Errorf("has:pr after unlink:\n%s", out)
	}
}
