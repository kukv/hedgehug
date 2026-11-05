package cli

import (
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"

	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// withPullRepo sets te up with a store on a fakegh, and makes a code
// repository the directory of the runs, whose origin is org-a/web on the fake
// and in a local bare repository, through url_rewrites.
func (te *testEnv) withPullRepo() *fakegh.Server {
	te.t.Helper()
	fake := te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	remotes := te.t.TempDir()
	gitrepotest.Bare(te.t, filepath.Join(remotes, "org-a", "web.git"))
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	te.env["HH_TOKEN"] = "t-alice"
	te.ok("store", "init", "me", "--prefix", "ME", "--account", "me")
	te.inCodeRepo()
	r, err := git.PlainOpen(te.dir)
	if err != nil {
		te.t.Fatal(err)
	}
	if _, err := r.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/org-a/web.git"}}); err != nil {
		te.t.Fatal(err)
	}
	return fake
}

func TestPRCommand(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	fake := te.withPullRepo()
	te.ok("add", "Fix the login")
	te.ok("start", "ME-1")
	if out := te.ok("pr", "ME-1", "--draft"); !strings.HasPrefix(out, "Opened org-a/web#1 for ME-1 ") {
		t.Errorf("pr:\n%s", out)
	}
	repo, _ := fake.Repo("org-a/web")
	if len(repo.Pulls) != 1 || !repo.Pulls[0].Draft {
		t.Fatalf("pulls = %+v", repo.Pulls)
	}
	if out := te.ok("show", "ME-1"); !strings.Contains(out, "pull request  org-a/web#1  draft") {
		t.Errorf("show:\n%s", out)
	}
	if out := te.ok("pr", "ME-1"); !strings.HasPrefix(out, "Pushed hh/ME-1-fix-the-login of ME-1 ") {
		t.Errorf("pr again:\n%s", out)
	}
}

func TestPRCommandLinksWhatIsOpen(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	fake := te.withPullRepo()
	te.ok("add", "a")
	te.ok("start", "ME-1")
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	if out := te.ok("pr", "ME-1"); !strings.HasPrefix(out, "Linked org-a/web#1 to ME-1 ") {
		t.Errorf("pr:\n%s", out)
	}
}

func TestPRWithNoBranchExits1(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.withPullRepo()
	te.ok("add", "a")
	if code, _, errs := te.run("pr", "ME-1"); code != 1 || !strings.Contains(errs, "hedgehug start ME-1") {
		t.Errorf("exit %d: %s", code, errs)
	}
}

func TestPRJSON(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.withPullRepo()
	te.ok("add", "a")
	te.ok("start", "ME-1")
	var w integration.PRWrite
	out := te.ok("pr", "ME-1", "--json")
	strictly(t, out, &w)
	if w.Ref != "github.com/org-a/web/pull/1" || w.Branch != "hh/ME-1-a" || !w.Created || w.Commit == nil {
		t.Errorf("json = %+v", w)
	}
	if w.Warnings == nil {
		t.Errorf("warnings is null:\n%s", out)
	}
}
