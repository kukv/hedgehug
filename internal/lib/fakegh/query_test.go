package fakegh_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// spy records the bodies of the requests that pass.
type spy struct{ bodies []string }

func (s *spy) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	s.bodies = append(s.bodies, string(b))
	r.Body = io.NopCloser(strings.NewReader(string(b)))
	return http.DefaultTransport.RoundTrip(r)
}

// realQueries are the queries internal/lib/github sends: ScanPulls, then
// LinkedItems.
func realQueries(t *testing.T) (scan, linked string) {
	t.Helper()
	s := fakegh.New(fakegh.State{
		Tokens: map[string]string{"t": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "o", Name: "r", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	var sp spy
	c := github.New(github.Options{API: srv.URL}, "t", &http.Client{Transport: &sp})
	if _, _, err := c.ScanPulls("o", "r", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.LinkedItems("o", "r", []int{1}, []int{2}); err != nil {
		t.Fatal(err)
	}
	pick := func(i int) string {
		var b struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(sp.bodies[i]), &b); err != nil {
			t.Fatal(err)
		}
		return b.Query
	}
	return pick(0), pick(1)
}

// ask posts query to the fake and returns the messages of "errors".
func ask(t *testing.T, query string) []string {
	t.Helper()
	s := fakegh.New(fakegh.State{
		Tokens: map[string]string{"t": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "o", Name: "r", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	// The pull request 1 and the issue 2 of the real queries are there.
	if _, err := s.OpenPull("o/r", "b", "t", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenIssue("o/r", "t", ""); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"query": query, "variables": map[string]any{"owner": "o", "name": "r"}})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/graphql", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer t")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	var msgs []string
	for _, e := range out.Errors {
		msgs = append(msgs, e.Message)
	}
	return msgs
}

func TestGraphQLChecksTheShapeOfTheQuery(t *testing.T) {
	t.Parallel()
	scan, linked := realQueries(t)
	if m := ask(t, scan); m != nil {
		t.Fatalf("scan: %v", m)
	}
	if m := ask(t, linked); m != nil {
		t.Fatalf("linked: %v", m)
	}
	for _, c := range []struct{ name, query, from, to string }{
		{"typo", scan, "reviewDecision", "reviewDecison"},
		{"missing field", scan, "latestOpinionatedReviews(first: 100) { nodes { state } }", ""},
		{"missing nested", scan, "statusCheckRollup { state }", "statusCheckRollup { }"},
		{"unknown field", scan, "headRefName", "headRefName authorAssociation"},
		{"no rateLimit", scan, "rateLimit { limit remaining resetAt }", ""},
		{"wrong order", scan, "direction: DESC", "direction: ASC"},
		{"wrong field order", scan, "field: UPDATED_AT", "field: CREATED_AT"},
		{"page size", scan, "pullRequests(first: 100", "pullRequests(first: 50"},
		{"comments size", scan, "comments(first: 100)", "comments(first: 10)"},
		{"stateReason", linked, "body state stateReason updatedAt", "body state updatedAt"},
		{"issue typo", linked, "stateReason", "stateReasons"},
		{"alias mismatch", linked, "p1: pullRequest(number: 1)", "p1: pullRequest(number: 3)"},
		{"alias kind", linked, "i2: issue(number: 2)", "i2: pullRequest(number: 2)"},
		{"alias form", linked, "p1:", "x1:"},
	} {
		q := strings.Replace(c.query, c.from, c.to, 1)
		if q == c.query {
			t.Fatalf("%s: %q is not in the query:\n%s", c.name, c.from, c.query)
		}
		if m := ask(t, q); len(m) == 0 {
			t.Errorf("%s: the fake accepted\n%s", c.name, q)
		}
	}
	if m := ask(t, "query Other { viewer { login } }"); len(m) == 0 {
		t.Error("unknown query accepted")
	}
}

func TestScanPullsStopsAtOnePageWithoutSince(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	for range 101 {
		if _, err := s.OpenPull("org-a/web", "b", "t", "", false); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := c.ScanPulls("org-a", "web", time.Time{})
	if err != nil || len(got) != 100 {
		t.Fatalf("%d pulls, %v", len(got), err)
	}
}

func TestReviewsInTheFake(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	n, _ := s.OpenPull("org-a/web", "b", "t", "", false)
	review := func() string {
		got, _, err := c.LinkedItems("org-a", "web", []int{n}, nil)
		if err != nil || len(got.Pulls) != 1 {
			t.Fatalf("%+v, %v", got, err)
		}
		return got.Pulls[0].Review
	}
	if r := review(); r != "none" {
		t.Errorf("fresh: %s", r)
	}
	_ = s.RequestReview("org-a/web", n)
	if r := review(); r != "pending" {
		t.Errorf("requested: %s", r)
	}
	_ = s.ReviewPull("org-a/web", n, "changes_requested")
	if r := review(); r != "changes_requested" {
		t.Errorf("changes: %s", r)
	}
	_ = s.ReviewPull("org-a/web", n, "approved")
	if r := review(); r != "approved" {
		t.Errorf("approved: %s", r)
	}
}
