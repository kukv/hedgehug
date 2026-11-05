package usecase_test

import (
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func stateOf(t *testing.T, ts *usecasetest.Session) ghstate.Repo {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	st, err := ghstate.Read(s)
	if err != nil {
		t.Fatal(err)
	}
	return st.Repos[web]
}

// counting passes the requests on, or answers status for all of them.
type counting struct {
	n      int
	status int
}

func (c *counting) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n++
	if c.status != 0 {
		return &http.Response{StatusCode: c.status, Status: http.StatusText(c.status), Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
	}
	return http.DefaultTransport.RoundTrip(r)
}

// GitHub's clock is behind this machine's: what was updated since the last
// read is told by GitHub's own times.
func TestFetchScansFromTheTimeGitHubGave(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	fake.SetNow(func() time.Time { return ts.Now().Add(-time.Hour) })
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	since := stateOf(t, ts).Since
	if !since.Equal(ts.Now().Add(-time.Hour)) {
		t.Fatalf("since = %v", since)
	}
	later(ts, 6*time.Minute)
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-b", "B", "", false); err != nil {
		t.Fatal(err)
	}
	if r := fetch(t, ts); !r.Committed {
		t.Fatal("the pull request opened after the last read was not read")
	}
	if got := stateOf(t, ts).Since; !got.After(since) {
		t.Errorf("since = %v, want after %v", got, since)
	}
	later(ts, 6*time.Minute)
	fetch(t, ts)
	if got := stateOf(t, ts).Since; !got.Equal(ts.Now().Add(-12*time.Minute - time.Hour + 6*time.Minute)) {
		t.Errorf("since moved with nothing updated: %v", got)
	}
}

func TestFetchDoesNotAskAgainAFailureWithinTheInterval(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		repo   string
		status int
		bad    bool // the token is not known
	}{
		"unauthorized": {repo: web, bad: true},
		"not found":    {repo: "github.com/org-b/secret"},
		"server error": {repo: web, status: 502},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts, fake := usecasetest.GitHubSession(t)
			if c.bad {
				ts.Env["HH_TOKEN"] = "unknown"
			}
			ts.AddTask("a", func(x *model.Task) { x.Repos = []string{c.repo} })
			ct := &counting{status: c.status}
			ts.HTTPClient = &http.Client{Transport: ct}
			first := fetch(t, ts)
			asked := ct.n
			if len(first.Warnings) != 1 || asked == 0 {
				t.Fatalf("report = %+v, requests = %d", first, asked)
			}
			later(ts, 4*time.Minute)
			again := fetch(t, ts)
			if ct.n != asked || !slices.Equal(again.Warnings, first.Warnings) {
				t.Fatalf("requests = %d (was %d), warnings = %q, want %q", ct.n, asked, again.Warnings, first.Warnings)
			}
			later(ts, time.Minute)
			fetch(t, ts)
			if ct.n == asked {
				t.Fatal("not asked again after five minutes")
			}
			// It reads, and the warning goes.
			ct.status = 0
			fake.AddToken("unknown", "alice")
			if c.repo == web {
				later(ts, 5*time.Minute)
				if r := fetch(t, ts); len(r.Warnings) != 0 {
					t.Fatalf("warnings = %q", r.Warnings)
				}
				if st := stateOf(t, ts); st.Warning != "" || !st.LastTry.Before(st.LastFetch) {
					t.Errorf("state = %+v", st)
				}
			}
		})
	}
}

// What a read found, an item that is not on GitHub, is told again until the
// next read, as a failure is, so that it does not fill the log of the GUI.
func TestFetchTellsAgainWhatTheLastReadFound(t *testing.T) {
	t.Parallel()
	ts, _ := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := integrationLinkIssue(ts, 7); err != nil {
		t.Fatal(err)
	}
	ct := &counting{}
	ts.HTTPClient = &http.Client{Transport: ct}
	first := fetch(t, ts)
	asked := ct.n
	if len(first.Warnings) != 1 || !strings.Contains(first.Warnings[0], "org-a/web#7") {
		t.Fatalf("warnings = %q", first.Warnings)
	}
	later(ts, time.Minute)
	again := fetch(t, ts)
	if ct.n != asked || !slices.Equal(again.Warnings, first.Warnings) {
		t.Errorf("requests = %d (was %d), warnings = %q", ct.n, asked, again.Warnings)
	}
}

func TestFetchReadsEveryPageOfTheScan(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "other", "first", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	later(ts, 6*time.Minute)
	for range 150 {
		if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
			t.Fatal(err)
		}
	}
	fetch(t, ts)
	if x := taskIn(t, ts, "me", "ME-1"); len(x.Links) != 150 {
		t.Errorf("links = %d, want 150", len(x.Links))
	}
}

// A write made while the fetch waits for the commit is kept: the fetch
// changes links and gh of the task as the store has it then.
func TestFetchKeepsAWriteMadeDuringIt(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	task := ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	ts.BeforeUpdate = func() {
		ts.BeforeUpdate = nil
		ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
			x, err := tx.Task(task.ULID)
			if err != nil {
				return err
			}
			x.Title = "changed meanwhile"
			return tx.PutTask(x)
		})
	}
	if r := fetch(t, ts); !r.Committed {
		t.Fatal("not committed")
	}
	x := taskIn(t, ts, "me", "ME-1")
	if x.Title != "changed meanwhile" || len(x.Links) != 1 {
		t.Errorf("title = %q, links = %+v", x.Title, x.Links)
	}
}

func integrationLinkIssue(ts *usecasetest.Session, n int) (usecase.TaskResult, error) {
	return integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "org-a/web#" + strconv.Itoa(n)})
}
