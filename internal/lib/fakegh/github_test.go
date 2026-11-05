package fakegh_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/github"
)

func client(t *testing.T) (*fakegh.Server, *github.Client) {
	t.Helper()
	s := fakegh.New(fakegh.State{
		Users:  []string{"alice"},
		Tokens: map[string]string{"t": "alice", "t2": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return s, github.New(github.Options{API: srv.URL}, "t", nil)
}

func TestPullLifecycle(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	n, err := s.OpenPull("org-a/web", "hh/WEB-1-a", "A", "body", true)
	if err != nil {
		t.Fatal(err)
	}
	pulls, _, err := c.ScanPulls("org-a", "web", time.Time{})
	if err != nil || len(pulls) != 1 || pulls[0].State != "draft" || pulls[0].Review != "none" || pulls[0].Head != "hh/WEB-1-a" {
		t.Fatalf("pulls = %+v, %v", pulls, err)
	}
	for _, f := range []func() error{
		func() error { return s.ReadyPull("org-a/web", n) },
		func() error { return s.RequestReview("org-a/web", n) },
		func() error { return s.SetChecks("org-a/web", n, "success") },
		func() error { return s.CommentPull("org-a/web", n, "see WEB-2") },
	} {
		if err := f(); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := c.LinkedItems("org-a", "web", []int{n}, nil)
	if err != nil || len(got.Pulls) != 1 {
		t.Fatalf("linked = %+v, %v", got, err)
	}
	p := got.Pulls[0]
	if p.State != "open" || p.Review != "pending" || p.Checks != "success" || len(p.Comments) != 1 {
		t.Errorf("pull = %+v", p)
	}
	if err := s.ReviewPull("org-a/web", n, "approved"); err != nil {
		t.Fatal(err)
	}
	if err := s.MergePull("org-a/web", n); err != nil {
		t.Fatal(err)
	}
	got, _, err = c.LinkedItems("org-a", "web", []int{n, n + 1}, nil)
	if err != nil || got.Pulls[0].State != "merged" || got.Pulls[0].Review != "approved" || got.Pulls[0].MergedAt == nil {
		t.Fatalf("merged = %+v, %v", got, err)
	}
	if len(got.MissingPulls) != 1 {
		t.Errorf("missing = %v", got.MissingPulls)
	}
}

func TestIssueLifecycle(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	n, err := s.OpenIssue("org-a/web", "I", "b")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue("org-a/web", n, "not_planned"); err != nil {
		t.Fatal(err)
	}
	got, _, err := c.LinkedItems("org-a", "web", nil, []int{n})
	if err != nil || len(got.Issues) != 1 || got.Issues[0].State != "closed" || got.Issues[0].StateReason != "not_planned" {
		t.Fatalf("issues = %+v, %v", got, err)
	}
	if err := s.EditIssue("org-a/web", n, "new", "c"); err != nil {
		t.Fatal(err)
	}
}

func TestGraphQLNumbersAndPaging(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	var last int
	for range 101 {
		n, err := s.OpenPull("org-a/web", "b", "t", "", false)
		if err != nil {
			t.Fatal(err)
		}
		last = n
	}
	i, err := s.OpenIssue("org-a/web", "I", "")
	if err != nil || i != last+1 {
		t.Fatalf("issue %d after pull %d, %v", i, last, err)
	}
	all, _, err := c.ScanPulls("org-a", "web", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(all) != 101 {
		t.Fatalf("%d pulls, %v", len(all), err)
	}
}

func TestGraphQLRateLimitAndAccess(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	s.SetRateLimit(50, 5000, at)
	if _, rl, err := c.ScanPulls("org-a", "web", time.Time{}); err != nil || rl.Remaining != 50 || !rl.ResetAt.Equal(at) {
		t.Errorf("rate limit = %+v, %v", rl, err)
	}
	if _, _, err := c.ScanPulls("org-a", "nothing", time.Time{}); err == nil {
		t.Error("no error for a missing repository")
	}
	if err := s.CloseIssue("org-a/web", 99, "completed"); err == nil {
		t.Error("no error for a missing issue")
	}
}
