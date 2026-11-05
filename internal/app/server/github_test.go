package server

import (
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

const webRepo = "github.com/org-a/web"

// newGitHubServer is the test server, not started, whose github.com is a
// fakegh with the repository org-a/web that the account me can write to.
func newGitHubServer(t *testing.T) (*testServer, *fakegh.Server) {
	t.Helper()
	ts := newTestServer(t)
	ts.uc.DropDefaultAccount()
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice"},
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	ts.uc.AppendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n" +
		"accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	ts.env["HH_TOKEN"] = "t-alice"
	return ts, fake
}

func (ts *testServer) sync() {
	ts.t.Helper()
	ts.call("POST", "/api/sync", nil, 200, nil)
}

func TestLinkAndUnlinkTask(t *testing.T) {
	t.Parallel()
	ts, fake := newGitHubServer(t)
	n, err := fake.OpenPull("org-a/web", "feature", "the pull", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id := ts.add("a", "")
	var out taskWriteBody
	ts.call("POST", "/api/tasks/"+id+"/links", map[string]any{"pr": "org-a/web#" + itoa(n)}, 200, &out)
	if out.Task.ID != id {
		t.Errorf("task = %+v", out.Task)
	}
	ts.sync()
	var got usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &got)
	if len(got.Links) != 1 || got.Links[0].Title != "the pull" || got.Links[0].Branch != "feature" || got.Links[0].State != "open" || got.Links[0].Via != "" {
		t.Fatalf("links = %+v", got.Links)
	}
	var list struct {
		Tasks []usecase.TaskView `json:"tasks"`
	}
	ts.call("GET", "/api/tasks", nil, 200, &list)
	if len(list.Tasks) != 1 || len(list.Tasks[0].Links) != 1 {
		t.Errorf("list = %+v", list.Tasks)
	}

	if p := ts.problem("POST", "/api/tasks/"+id+"/links", map[string]any{"pr": "org-a/web#" + itoa(n)}, 422); p.Type != "link.exists" {
		t.Errorf("twice = %+v", p)
	}
	if p := ts.problem("POST", "/api/tasks/"+id+"/links", map[string]any{}, 400); p.Type == "" {
		t.Errorf("nothing = %+v", p)
	}
	if p := ts.problem("POST", "/api/tasks/"+id+"/links", map[string]any{"pr": "a/b#1", "issue": "a/b#2"}, 400); p.Type == "" {
		t.Errorf("both = %+v", p)
	}
	if p := ts.problem("POST", "/api/tasks/NOPE-1/links", map[string]any{"pr": "a/b#1"}, 404); p.Type != "task.not_found" {
		t.Errorf("no task = %+v", p)
	}

	path := "/api/tasks/" + id + "/links?pr=" + url.QueryEscape("org-a/web#"+itoa(n))
	ts.call("DELETE", path, nil, 200, &out)
	var after usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &after)
	if len(after.Links) != 0 {
		t.Errorf("after unlink = %+v", after.Links)
	}
	if p := ts.problem("DELETE", path, nil, 422); p.Type != "link.not_found" {
		t.Errorf("unlink twice = %+v", p)
	}
	if p := ts.problem("DELETE", "/api/tasks/"+id+"/links", nil, 400); p.Type == "" {
		t.Errorf("nothing = %+v", p)
	}
}

func TestLinkChangesTheModeOfAnIssue(t *testing.T) {
	t.Parallel()
	ts, fake := newGitHubServer(t)
	n, err := fake.OpenIssue("org-a/web", "the issue", "")
	if err != nil {
		t.Fatal(err)
	}
	id := ts.add("a", "")
	ref := "org-a/web#" + itoa(n)
	ts.call("POST", "/api/tasks/"+id+"/links", map[string]any{"issue": ref, "sync": "link"}, 200, nil)
	ts.call("POST", "/api/tasks/"+id+"/links", map[string]any{"issue": ref, "sync": "mirror"}, 200, nil)
	var got usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &got)
	if len(got.Links) != 1 || got.Links[0].Sync != "mirror" {
		t.Errorf("links = %+v", got.Links)
	}
	ts.problem("POST", "/api/tasks/"+id+"/links", map[string]any{"issue": ref, "sync": "off"}, 400)
}

func TestCreateIssue(t *testing.T) {
	t.Parallel()
	ts, fake := newGitHubServer(t)
	id := ts.add("a", "")
	if p := ts.problem("POST", "/api/tasks/"+id+"/issue", map[string]any{}, 422); p.Type != "issue.repo_required" {
		t.Errorf("no repository = %+v", p)
	}
	var out integration.IssueWrite
	ts.call("POST", "/api/tasks/"+id+"/issue", map[string]any{"repo": "org-a/web"}, 201, &out)
	if out.Ref != webRepo+"/issues/1" {
		t.Errorf("ref = %s", out.Ref)
	}
	r, ok := fake.Repo("org-a/web")
	if !ok || len(r.Issues) != 1 || r.Issues[0].Title != "a" {
		t.Errorf("repo = %+v", r)
	}
	var got usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &got)
	if len(got.Links) != 1 || got.Links[0].Sync != "push" || got.Links[0].Title != "a" {
		t.Errorf("links = %+v", got.Links)
	}
	if p := ts.problem("POST", "/api/tasks/"+id+"/issue", map[string]any{"repo": "org-a/web"}, 422); p.Type != "link.exists" {
		t.Errorf("twice = %+v", p)
	}
}

func TestImportIssues(t *testing.T) {
	t.Parallel()
	ts, fake := newGitHubServer(t)
	if err := fake.AddLabel("org-a/web", "v2"); err != nil {
		t.Fatal(err)
	}
	n, err := fake.OpenIssue("org-a/web", "from github", "the body")
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.TagIssue("org-a/web", n, nil, []string{"v2"}, ""); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"repo": "org-a/web", "label": "v2", "dry_run": true}
	var out integration.ImportResult
	ts.call("POST", "/api/github/import", body, 200, &out)
	if out.Count != 1 || len(out.Created) != 0 || !out.DryRun {
		t.Fatalf("dry run = %+v", out)
	}
	delete(body, "dry_run")
	ts.call("POST", "/api/github/import", body, 200, &out)
	if len(out.Created) != 1 {
		t.Fatalf("import = %+v", out)
	}
	ts.call("POST", "/api/github/import", body, 200, &out)
	if out.Count != 0 || out.Skipped != 1 {
		t.Errorf("again = %+v", out)
	}
	ts.problem("POST", "/api/github/import", map[string]any{"repo": "org-a/web"}, 400)
}

func TestGetGitHub(t *testing.T) {
	t.Parallel()
	ts, fake := newGitHubServer(t)
	var out []usecase.GitHubRepoView
	ts.call("GET", "/api/github", nil, 200, &out)
	if len(out) != 0 {
		t.Fatalf("no repository = %+v", out)
	}
	id := ts.add("a", "")
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"repos": []string{webRepo}}, 200, nil)
	ts.call("GET", "/api/github?store=me", nil, 200, &out)
	if len(out) != 1 || out[0].Repo != webRepo || out[0].Store != "me" || out[0].Account != "me" || out[0].LastFetch != nil {
		t.Fatalf("before = %+v", out)
	}
	fake.RestrictApp("org-a")
	ts.sync()
	ts.call("GET", "/api/github", nil, 200, &out)
	if len(out) != 1 || !out[0].AppMissing || len(out[0].Warnings) != 1 || out[0].NextFetch == nil {
		t.Fatalf("restricted = %+v", out)
	}
	var sts []usecase.SyncStatusView
	ts.call("GET", "/api/sync", nil, 200, &sts)
	if len(sts) != 1 || sts[0].GitHub == nil || sts[0].GitHub.AppMissing != 1 {
		t.Fatalf("sync = %+v", sts)
	}
}

func TestGetGitHubAfterARead(t *testing.T) {
	t.Parallel()
	ts, _ := newGitHubServer(t)
	id := ts.add("a", "")
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"repos": []string{webRepo}}, 200, nil)
	ts.sync()
	var out []usecase.GitHubRepoView
	ts.call("GET", "/api/github", nil, 200, &out)
	if len(out) != 1 || out[0].AppMissing || out[0].LastFetch == nil || out[0].NextFetch == nil || out[0].Limit == 0 {
		t.Errorf("read = %+v", out)
	}
}

func TestRepoAccounts(t *testing.T) {
	t.Parallel()
	ts, _ := newGitHubServer(t)
	var out repoAccountsBody
	ts.call("GET", "/api/repo-accounts", nil, 200, &out)
	if len(out.Rules) != 0 {
		t.Fatalf("empty = %+v", out)
	}
	rules := []accounts.RepoAccountView{{Match: "github.com/org-a/*", Account: "me"}, {Match: "*", Account: "me"}}
	ts.call("PUT", "/api/repo-accounts", map[string]any{"rules": rules}, 200, &out)
	if !slices.Equal(out.Rules, rules) {
		t.Fatalf("put = %+v", out)
	}
	ts.call("GET", "/api/repo-accounts", nil, 200, &out)
	if !slices.Equal(out.Rules, rules) {
		t.Fatalf("get = %+v", out)
	}
	bad := map[string]any{"rules": []map[string]string{{"match": "*", "account": "nobody"}}}
	if p := ts.problem("PUT", "/api/repo-accounts", bad, 422); p.Type != "repo_accounts.invalid" || !strings.Contains(p.Detail, "nobody") {
		t.Errorf("unknown account = %+v", p)
	}
	ts.problem("PUT", "/api/repo-accounts", map[string]any{"rules": nil}, 400)
}

func TestAccountTokenSource(t *testing.T) {
	t.Parallel()
	ts, _ := newGitHubServer(t)
	var out accounts.AuthSourceResult
	ts.call("PUT", "/api/accounts/me/token-source", map[string]any{"token": "env:OTHER"}, 200, &out)
	if out.Token != "env:OTHER" {
		t.Errorf("source = %+v", out)
	}
	if p := ts.problem("PUT", "/api/accounts/me/token-source", map[string]any{"token": "keychain"}, 400); p.Type != "auth.token_source" {
		t.Errorf("keychain = %+v", p)
	}
	if p := ts.problem("PUT", "/api/accounts/nobody/token-source", map[string]any{"token": "gh"}, 404); p.Type != "account.not_found" {
		t.Errorf("nobody = %+v", p)
	}
}
