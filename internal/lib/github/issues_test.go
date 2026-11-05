package github_test

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/github"
)

func issueFake(t *testing.T) (*fakegh.Server, *github.Client) {
	t.Helper()
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice", "bob", "carol"},
		Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"},
		Repos: []*fakegh.Repo{{
			Owner: "alice", Name: "web", Perms: map[string]string{"bob": "read"},
			Labels: []string{"bug", "api"}, Milestones: []string{"v1", "v2"},
		}},
	})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return fake, github.New(github.Options{API: srv.URL}, "t-alice", nil)
}

func issueOf(t *testing.T, fake *fakegh.Server, n int) *fakegh.Issue {
	t.Helper()
	r, _ := fake.Repo("alice/web")
	for _, x := range r.Issues {
		if x.Number == n {
			return x
		}
	}
	t.Fatalf("no issue %d", n)
	return nil
}

func TestCreateAndUpdateIssue(t *testing.T) {
	t.Parallel()
	fake, c := issueFake(t)
	n, err := c.CreateIssue("alice", "web", github.NewIssue{Title: "a", Body: "b", Assignees: []string{"bob"}, Labels: []string{"bug"}, Milestone: 2})
	if err != nil {
		t.Fatal(err)
	}
	x := issueOf(t, fake, n)
	if x.Title != "a" || x.Body != "b" || x.State != "open" || !slices.Equal(x.Assignees, []string{"bob"}) || !slices.Equal(x.Labels, []string{"bug"}) || x.Milestone != "v2" {
		t.Errorf("created %+v", x)
	}
	title, closed, reason, none := "c", "closed", "not_planned", 0
	empty := []string{}
	if err := c.UpdateIssue("alice", "web", n, github.IssueUpdate{Title: &title, State: &closed, StateReason: &reason, Labels: &empty, Milestone: &none}); err != nil {
		t.Fatal(err)
	}
	x = issueOf(t, fake, n)
	if x.Title != "c" || x.Body != "b" || x.State != "closed" || x.StateReason != "not_planned" || len(x.Labels) != 0 || x.Milestone != "" || len(x.Assignees) != 1 {
		t.Errorf("updated %+v", x)
	}
	open := "open"
	if err := c.UpdateIssue("alice", "web", n, github.IssueUpdate{State: &open}); err != nil {
		t.Fatal(err)
	}
	if x = issueOf(t, fake, n); x.State != "open" || x.StateReason != "" {
		t.Errorf("reopened %+v", x)
	}
}

func TestIssueWritesThatGitHubRefuses(t *testing.T) {
	t.Parallel()
	_, c := issueFake(t)
	for name, in := range map[string]github.NewIssue{
		"label":    {Title: "a", Labels: []string{"nope"}},
		"assignee": {Title: "a", Assignees: []string{"carol"}},
	} {
		if _, err := c.CreateIssue("alice", "web", in); err == nil || !strings.Contains(err.Error(), "422") {
			t.Errorf("%s: %v", name, err)
		}
	}
	title := "x"
	if err := c.UpdateIssue("alice", "web", 99, github.IssueUpdate{Title: &title}); !errors.Is(err, github.ErrNotFound) {
		t.Errorf("no issue: %v", err)
	}
}

func TestIssueWriteNeedsWriteAccess(t *testing.T) {
	t.Parallel()
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice", "bob"},
		Tokens: map[string]string{"t-bob": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "alice", Name: "web", Perms: map[string]string{"bob": "read"}}},
	})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	c := github.New(github.Options{API: srv.URL}, "t-bob", nil)
	if _, err := c.CreateIssue("alice", "web", github.NewIssue{Title: "a"}); !errors.Is(err, github.ErrForbidden) {
		t.Errorf("create: %v", err)
	}
}

func TestRepositoryNames(t *testing.T) {
	t.Parallel()
	_, c := issueFake(t)
	if got, err := c.Labels("alice", "web"); err != nil || !slices.Equal(got, []string{"bug", "api"}) {
		t.Errorf("labels %v, %v", got, err)
	}
	if got, err := c.Assignees("alice", "web"); err != nil || !slices.Equal(got, []string{"alice", "bob"}) {
		t.Errorf("assignees %v, %v", got, err)
	}
	if got, err := c.Milestones("alice", "web"); err != nil || len(got) != 2 || got[1] != (github.Milestone{Number: 2, Title: "v2"}) {
		t.Errorf("milestones %v, %v", got, err)
	}
}

func TestLinkedItemsReadTheFieldsOfAnIssue(t *testing.T) {
	t.Parallel()
	fake, c := issueFake(t)
	n, err := fake.OpenIssue("alice/web", "t", "b")
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.TagIssue("alice/web", n, []string{"bob"}, []string{"bug"}, "v1"); err != nil {
		t.Fatal(err)
	}
	got, _, err := c.LinkedItems("alice", "web", nil, []int{n})
	if err != nil {
		t.Fatal(err)
	}
	x := got.Issues[0]
	if x.Title != "t" || x.Body != "b" || !slices.Equal(x.Assignees, []string{"bob"}) || !slices.Equal(x.Labels, []string{"bug"}) || x.Milestone != "v1" {
		t.Errorf("issue %+v", x)
	}
}

func TestOpenIssues(t *testing.T) {
	t.Parallel()
	fake, c := issueFake(t)
	var want []int
	for i := range 120 {
		n, err := fake.OpenIssue("alice/web", fmt.Sprintf("i%d", i), "")
		if err != nil {
			t.Fatal(err)
		}
		if i%10 != 9 {
			if err := fake.TagIssue("alice/web", n, nil, []string{"bug"}, ""); err != nil {
				t.Fatal(err)
			}
			want = append(want, n)
		}
	}
	if _, err := fake.OpenPull("alice/web", "x", "pr", "", false); err != nil {
		t.Fatal(err)
	}
	if err := fake.CloseIssue("alice/web", want[0], "completed"); err != nil {
		t.Fatal(err)
	}
	want = want[1:]
	got, rl, err := c.OpenIssues("alice", "web", "bug")
	if err != nil || rl.Limit == 0 {
		t.Fatalf("%v, %+v", err, rl)
	}
	nums := make([]int, len(got))
	for i, x := range got {
		nums[i] = x.Number
	}
	if !slices.Equal(nums, want) {
		t.Errorf("got %v", nums)
	}
}
