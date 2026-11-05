package usecase_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"go.yaml.in/yaml/v4"
)

const web = "github.com/org-a/web"

func fetch(t *testing.T, ts *usecasetest.Session) usecase.GitHubFetchReport {
	t.Helper()
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	r, err := usecase.FetchGitHub(ts.Session, c, e)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func later(ts *usecasetest.Session, d time.Duration) {
	now := ts.Now().Add(d)
	ts.Now = func() time.Time { return now }
}

func TestFetchLinksByBranchAndWritesState(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.RequestReview("org-a/web", n); err != nil {
		t.Fatal(err)
	}
	r := fetch(t, ts)
	if !r.Committed || len(r.Warnings) != 0 {
		t.Fatalf("report = %+v", r)
	}
	x := taskIn(t, ts, "me", "ME-1")
	g, _ := x.GitHub()
	if len(x.Links) != 1 || len(g.PRs) != 1 || g.PRs[0].State != "open" || g.PRs[0].Review != "pending" {
		t.Fatalf("links = %+v, gh = %+v", x.Links, g)
	}
	if usecasetest.LastOp(t, ts) != "gh-fetch" {
		t.Errorf("op = %s", usecasetest.LastOp(t, ts))
	}
}

func TestFetchKeepsTheInterval(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#1"}); err != nil {
		t.Fatal(err)
	}
	n, err := fake.OpenPull("org-a/web", "b", "A", "", false)
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	fetch(t, ts)
	if err := fake.MergePull("org-a/web", 1); err != nil {
		t.Fatal(err)
	}
	later(ts, 4*time.Minute)
	if r := fetch(t, ts); r.Committed {
		t.Fatal("read within five minutes")
	}
	later(ts, time.Minute)
	if r := fetch(t, ts); !r.Committed {
		t.Fatal("not read after five minutes")
	}
	g, _ := taskIn(t, ts, "me", "ME-1").GitHub()
	if g.PRs[0].State != "merged" {
		t.Errorf("prs = %+v", g.PRs)
	}
	later(ts, 10*time.Minute)
	if r := fetch(t, ts); r.Committed {
		t.Error("committed with nothing new")
	}
}

func TestFetchSlowsDownNearTheRateLimit(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	fake.SetRateLimit(10, 5000, ts.Now().Add(time.Hour))
	fetch(t, ts)
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	later(ts, 14*time.Minute)
	if r := fetch(t, ts); r.Committed {
		t.Fatal("read within fifteen minutes")
	}
	later(ts, time.Minute)
	if r := fetch(t, ts); !r.Committed {
		t.Fatal("not read after fifteen minutes")
	}
}

func TestFetchDoesNotLinkBackWhatWasUnlinked(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	if _, err := integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#1"}); err != nil {
		t.Fatal(err)
	}
	if err := fake.CommentPull("org-a/web", 1, "ping"); err != nil {
		t.Fatal(err)
	}
	later(ts, time.Hour)
	fetch(t, ts)
	x := taskIn(t, ts, "me", "ME-1")
	g, _ := x.GitHub()
	if len(x.Links) != 1 || x.Links[0].Sync != model.SyncOff || len(g.PRs) != 0 {
		t.Fatalf("links = %+v, gh = %+v", x.Links, g)
	}
}

func TestFetchWarnsAndGoesOn(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{"github.com/org-b/secret", web} })
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "org-a/web#7"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r := fetch(t, ts)
	if len(r.Warnings) != 2 || !strings.Contains(r.Warnings[0], "org-a/web#7") || !strings.Contains(r.Warnings[1], "org-b/secret") {
		t.Fatalf("warnings = %q", r.Warnings)
	}
	if !r.Committed {
		t.Error("the repository that reads is not written")
	}
}

func TestFetchSkipsAReadOnlyStore(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	usecasetest.MakeReadOnly(t, ts, "no push access")
	if r := fetch(t, ts); r.Committed || len(r.Warnings) != 0 {
		t.Fatalf("report = %+v", r)
	}
}

// A member who reads what another one wrote already has nothing to write.
// The state of this machine is lost, as on a clone of its own.
func TestFetchOfTheSameValueWritesNothing(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal("github.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if r := fetch(t, ts); r.Committed {
		t.Error("wrote the same value again")
	}
}

func lastFetch(t *testing.T, ts *usecasetest.Session, store, repo string) time.Time {
	t.Helper()
	s, err := ts.ForStore(store).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	st, err := ghstate.Read(s)
	if err != nil {
		t.Fatal(err)
	}
	return st.Repos[repo].LastFetch
}

// The time moves only for the repository that was read.
func TestFetchAdvancesTheTimeOnlyOfWhatWasRead(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{"github.com/org-b/secret", web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	if got := lastFetch(t, ts, "me", web); !got.Equal(ts.Now()) {
		t.Errorf("read repository = %v, want %v", got, ts.Now())
	}
	if got := lastFetch(t, ts, "me", "github.com/org-b/secret"); !got.IsZero() {
		t.Errorf("repository that failed = %v, want zero", got)
	}
}

// When what was found cannot be written to a task, the repository is not
// marked as read, so that the automatic link is not lost with the time.
func TestFetchKeepsTheTimeWhenATaskCannotBeWritten(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) {
		x.Repos = []string{web}
		x.GH = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "text"}
	})
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r := fetch(t, ts)
	if len(r.Warnings) != 1 || r.Committed {
		t.Fatalf("report = %+v", r)
	}
	if got := lastFetch(t, ts, "me", web); !got.IsZero() {
		t.Errorf("time = %v, want zero", got)
	}
}

// Two stores that read the same pull request each link only their own tasks,
// and each keeps its own time.
func TestFetchOfAPullRequestAcrossStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	fake := fakegh.New(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Users:  []string{"alice", "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	fake.SetNow(ts.Now)
	ts.Env["HH_TOKEN"] = "t-alice"
	ts.AppendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alice", "bob"} {
		if err := c.SetAccountToken(name, config.TokenSource{Kind: "env", Env: "HH_TOKEN"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	// Both tasks name the repository, so both stores read it.
	for _, arg := range []string{"org-a:API-1", "org-b:WEB-1"} {
		if _, err := integration.Link(ts.Session, arg, integration.LinkInput{PR: "org-a/web#2"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fake.OpenPull("org-a/web", "hh/API-1-x", "WEB-1 too", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.OpenPull("org-a/web", "other", "shared", "", false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"org-a", "org-b"} {
		c, e, err := ts.ForStore(name).Entry()
		if err != nil {
			t.Fatal(err)
		}
		r, err := usecase.FetchGitHub(ts.Session, c, e)
		if err != nil || !r.Committed || len(r.Warnings) != 0 {
			t.Fatalf("%s: report = %+v, %v", name, r, err)
		}
	}
	for _, w := range []struct {
		store, id string
		n         int // links
	}{{"org-a", "API-1", 2}, {"org-b", "WEB-1", 2}} {
		x := taskIn(t, ts, w.store, w.id)
		g, _ := x.GitHub()
		if len(x.Links) != w.n || len(g.PRs) != w.n {
			t.Errorf("%s: links = %+v, gh = %+v", w.id, x.Links, g)
		}
		if !lastFetch(t, ts, w.store, web).Equal(ts.Now()) {
			t.Errorf("%s: no time", w.store)
		}
	}
}

func taskIn(t *testing.T, ts *usecasetest.Session, store, id string) *model.Task {
	t.Helper()
	s, ix, err := ts.ForStore(store).Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ts.Resolve(ix, id)
	if err != nil {
		t.Fatal(err)
	}
	x, err := s.Task(e.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
