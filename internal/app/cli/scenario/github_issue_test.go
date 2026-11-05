package scenario

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// shown is what hedgehug show --json says of a task.
type shown struct {
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Assignees []string `json:"assignees"`
	Links     []struct {
		Ref   string `json:"ref"`
		State string `json:"state"`
	} `json:"links"`
}

func (u *ghUser) show(id string) shown {
	u.t.Helper()
	var x shown
	if err := json.Unmarshal([]byte(u.ok("show", id, "--json")), &x); err != nil {
		u.t.Fatalf("show %s: %v", id, err)
	}
	return x
}

// TestGitHubIssueSync follows the three modes and the bulk import with the
// built command, and sees a hand edit on either side told in the inbox.
func TestGitHubIssueSync(t *testing.T) {
	t.Parallel()
	bin := buildHedgehug(t)
	fake := fakegh.New(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Users:  []string{"alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}, Labels: []string{"hh"}}},
	})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	root := t.TempDir()
	home := filepath.Join(root, "hh")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := "accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	u := &ghUser{t: t, bin: bin, dir: root}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HOME=") && !strings.HasPrefix(kv, "HEDGEHUG_") && !strings.HasPrefix(kv, "GIT_CONFIG_") {
			u.env = append(u.env, kv)
		}
	}
	u.env = append(u.env, "HEDGEHUG_HOME="+home, "HOME="+home, "HEDGEHUG_AUTO_SYNC=0", "HH_TOKEN=t-alice")
	u.ok("store", "init", "me", "--prefix", "WEB", "--account", "me")
	issue := func(n int) *fakegh.Issue {
		t.Helper()
		r, _ := fake.Repo("org-a/web")
		for _, x := range r.Issues {
			if x.Number == n {
				return x
			}
		}
		t.Fatalf("no issue %d", n)
		return nil
	}
	sync := func() {
		t.Helper()
		ageFetches(t, home, "me")
		u.ok("sync")
	}

	// push: the task goes to GitHub, and a hand edit there is written over.
	u.ok("add", "Pushed", "--store", "me")
	u.ok("set", "WEB-1", "repos=github.com/org-a/web", "assignees+=github.com/alice")
	if out := u.ok("issue", "WEB-1"); !strings.Contains(out, "Opened org-a/web#1 for WEB-1") {
		t.Fatalf("issue: %s", out)
	}
	if x := issue(1); x.Title != "Pushed" || !slices.Equal(x.Assignees, []string{"alice"}) || !strings.Contains(x.Body, "hedgehug:") {
		t.Fatalf("issue %+v", x)
	}
	u.ok("sync") // the first fetch: it starts the inbox of a store with no remote
	u.ok("set", "WEB-1", "title=Pushed again")
	sync()
	if x := issue(1); x.Title != "Pushed again" {
		t.Fatalf("issue %+v", x)
	}
	if err := fake.EditIssue("org-a/web", 1, "Edited on GitHub", ""); err != nil {
		t.Fatal(err)
	}
	sync()
	if x := issue(1); x.Title != "Pushed again" {
		t.Fatalf("the edit stayed: %+v", x)
	}
	inbox := u.ok("inbox")
	if !strings.Contains(inbox, `the title "Edited on GitHub" on org-a/web#1 was written over by the task`) {
		t.Fatalf("inbox:\n%s", inbox)
	}
	u.ok("done", "WEB-1")
	sync()
	if x := issue(1); x.State != "closed" || x.StateReason != "completed" {
		t.Fatalf("issue %+v", x)
	}

	// import and mirror: GitHub comes to the task, and a hand edit is put back.
	n, err := fake.OpenIssue("org-a/web", "Imported", "their text")
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.TagIssue("org-a/web", n, []string{"alice"}, []string{"hh"}, ""); err != nil {
		t.Fatal(err)
	}
	if out := u.ok("gh", "import", "org-a/web", "--label", "hh", "--dry-run"); !strings.Contains(out, "Would import 1 issue") {
		t.Fatalf("dry run: %s", out)
	}
	if out := u.ok("gh", "import", "org-a/web", "--label", "hh"); !strings.Contains(out, "WEB-2") {
		t.Fatalf("import: %s", out)
	}
	if x := u.show("WEB-2"); x.Title != "Imported" || !slices.Equal(x.Assignees, []string{"github.com/alice"}) {
		t.Fatalf("task %+v", x)
	}
	u.ok("set", "WEB-2", "title=Mine")
	sync()
	if x := u.show("WEB-2"); x.Title != "Imported" {
		t.Fatalf("task %+v", x)
	}
	if inbox := u.ok("inbox"); !strings.Contains(inbox, `the title "Mine" of the task was written over by org-a/web#`+strconv.Itoa(n)) {
		t.Fatalf("inbox:\n%s", inbox)
	}
	if err := fake.CloseIssue("org-a/web", n, "completed"); err != nil {
		t.Fatal(err)
	}
	sync()
	if x := u.show("WEB-2"); x.Status != "done" {
		t.Fatalf("task %+v", x)
	}

	// link: only the state comes in.
	m, err := fake.OpenIssue("org-a/web", "Linked", "")
	if err != nil {
		t.Fatal(err)
	}
	u.ok("add", "Only a link", "--store", "me")
	u.ok("link", "WEB-3", "--issue", "org-a/web#"+strconv.Itoa(m), "--sync", "link")
	if err := fake.CloseIssue("org-a/web", m, "completed"); err != nil {
		t.Fatal(err)
	}
	sync()
	x := u.show("WEB-3")
	if x.Title != "Only a link" || x.Status != "todo" || len(x.Links) != 1 || x.Links[0].State != "closed" {
		t.Fatalf("task %+v", x)
	}
}
