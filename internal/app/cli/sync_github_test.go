package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// githubEnv is a store with no remote whose task is linked to a pull request
// that exists on the fake.
func githubEnv(t *testing.T) (te *testEnv, ulid string) {
	t.Helper()
	te = newStoreEnv(t)
	fake := te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	te.writeConfigYAML("accounts:\n  - {name: alice, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.env["HH_TOKEN"] = "t-alice"
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	a := te.addJSON("a")
	te.ok("link", a.ID, "--pr", "org-a/web#1")
	return te, a.ULID
}

func TestSyncWithoutArgumentsReadsGitHubForAStoreWithoutRemote(t *testing.T) {
	t.Parallel()
	te, ulid := githubEnv(t)
	out := te.ok("sync")
	if strings.Contains(out, "Nothing to sync") || !strings.Contains(out, "Read GitHub for me") {
		t.Errorf("sync = %q", out)
	}
	g, err := te.task(ulid).GitHub()
	if err != nil || len(g.PRs) != 1 || g.PRs[0].State != "open" {
		t.Errorf("gh = %+v, %v", g, err)
	}
}

func TestSyncSaysGitHubWasReadWhenNothingChanged(t *testing.T) {
	t.Parallel()
	te, _ := githubEnv(t)
	te.ok("sync")
	te.now = te.now.Add(5 * time.Minute) // GitHub is read every 5 minutes
	if out := te.ok("sync"); !strings.Contains(out, "Read GitHub for me: nothing new (no remote, nothing to push)") {
		t.Errorf("sync = %q", out)
	}
}

func TestSyncSaysNothingWhenGitHubIsNotDue(t *testing.T) {
	t.Parallel()
	te, _ := githubEnv(t)
	te.ok("sync")
	if out := te.ok("sync"); !strings.Contains(out, "Nothing to sync: me has no remote") {
		t.Errorf("sync = %q", out)
	}
}

func TestSyncJSONSaysGitHubWasRead(t *testing.T) {
	t.Parallel()
	te, _ := githubEnv(t)
	var reps []usecase.SyncReport
	if err := json.Unmarshal([]byte(te.ok("sync", "--json")), &reps); err != nil || len(reps) != 1 || !reps[0].NoRemote || !reps[0].GitHubUpdated {
		t.Errorf("reports = %+v, %v", reps, err)
	}
}
