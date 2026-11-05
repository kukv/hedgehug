package scenario

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// ghUser runs hedgehug in the code repository dir, with its own home and a
// token for the fake GitHub.
type ghUser struct {
	t   *testing.T
	bin string
	dir string
	env []string
}

func (u *ghUser) ok(args ...string) string {
	u.t.Helper()
	cmd := exec.Command(u.bin, args...) //nolint:gosec // the binary the test built
	cmd.Env, cmd.Dir = u.env, u.dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		u.t.Fatalf("hedgehug %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// status is the status of WEB-1.
func (u *ghUser) status() string {
	u.t.Helper()
	var x struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(u.ok("show", "WEB-1", "--json")), &x); err != nil {
		u.t.Fatalf("show WEB-1: %v", err)
	}
	return x.Status
}

// ageFetches sets last_fetch of every repository in the GitHub state of the
// store to an hour ago, so that the interval of 5 minutes has passed.
func ageFetches(t *testing.T, home, store string) {
	t.Helper()
	p := filepath.Join(home, "stores", store, ".hedgehug", "local", "github.json")
	b, err := os.ReadFile(p) //nolint:gosec // the state file of the test
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Repos map[string]map[string]any `json:"repos"`
	}
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Repos) == 0 {
		t.Fatalf("no repository in %s", p)
	}
	for _, r := range st.Repos {
		r["last_fetch"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	}
	if b, err = json.Marshal(st); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestGitHubAutomationFlow follows the completion flow of phase 5b with the
// built command: start, pr, a review request and the merge move the task to
// in_progress, review and done, with no hedgehug status by hand.
func TestGitHubAutomationFlow(t *testing.T) {
	t.Parallel()
	bin := buildHedgehug(t)
	fake := fakegh.New(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Users:  []string{"alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	root := t.TempDir()
	home := filepath.Join(root, "hh")
	remotes := filepath.Join(root, "remotes")
	code := filepath.Join(root, "web")
	for _, d := range []string{home, remotes, code} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	cfg := "accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n" +
		"url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	gitrepotest.Bare(t, filepath.Join(remotes, "org-a", "web.git"))

	// The code repository: made with go-git only, with origin on GitHub.
	r, err := git.PlainInitWithOptions(code, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/org-a/web.git"}}); err != nil {
		t.Fatal(err)
	}
	commit := func(file, subject string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(code, file), []byte(file+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitrepotest.CommitFiles(t, code, subject, "alice", "alice@example.com", time.Now(), file)
	}
	commit("a.txt", "first")

	u := &ghUser{t: t, bin: bin, dir: code}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HOME=") && !strings.HasPrefix(kv, "HEDGEHUG_") && !strings.HasPrefix(kv, "GIT_CONFIG_") {
			u.env = append(u.env, kv)
		}
	}
	u.env = append(u.env, "HEDGEHUG_HOME="+home, "HOME="+home, "HEDGEHUG_AUTO_SYNC=0", "HH_TOKEN=t-alice")

	u.ok("store", "init", "me", "--prefix", "WEB", "--account", "me", "--template", "dev", "--remote", "https://github.com/org-a/hh-tasks.git")
	u.ok("sync")
	u.ok("add", "Fix the login")
	if got := u.status(); got != "todo" {
		t.Fatalf("status after add = %q, want todo", got)
	}

	// hh start: the branch, and in_progress.
	if out := u.ok("start", "WEB-1"); !strings.Contains(out, "On branch hh/WEB-1-fix-the-login") {
		t.Errorf("start:\n%s", out)
	}
	if got := u.status(); got != "in_progress" {
		t.Errorf("status after start = %q, want in_progress", got)
	}

	// Commit on the branch (start put HEAD on it), then hh pr.
	commit("b.txt", "fix the login")
	if out := u.ok("pr", "WEB-1"); !strings.HasPrefix(out, "Opened org-a/web#1 for WEB-1 ") {
		t.Errorf("pr:\n%s", out)
	}
	if got := u.status(); got != "in_progress" {
		t.Errorf("status after pr = %q, want in_progress", got)
	}

	// A review request, then the sync past the interval: review.
	if err := fake.RequestReview("org-a/web", 1); err != nil {
		t.Fatal(err)
	}
	ageFetches(t, home, "me")
	u.ok("sync", "--store", "me")
	if got := u.status(); got != "review" {
		t.Errorf("status after the review request = %q, want review", got)
	}

	// The merge, then the sync: done.
	if err := fake.MergePull("org-a/web", 1); err != nil {
		t.Fatal(err)
	}
	ageFetches(t, home, "me")
	u.ok("sync", "--store", "me")
	if got := u.status(); got != "done" {
		t.Errorf("status after the merge = %q, want done", got)
	}

	// The two moves the rules made are told in the inbox. pr-opened found
	// the task in in_progress already (hh start did that) and made no move,
	// so it tells nothing.
	out := u.ok("inbox")
	if strings.Count(out, "automation") != 2 || !strings.Contains(out, "rule pr-review moved it to review (pr.review_requested)") ||
		!strings.Contains(out, "rule pr-merged moved it to done (pr.merged)") {
		t.Errorf("inbox:\n%s", out)
	}
}
