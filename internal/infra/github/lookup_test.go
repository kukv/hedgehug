package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// graphQL は、POST /graphql の本文の変数を読み、answer(owner, name, number) の JSON を返す偽のサーバーの関数。
func graphQL(t *testing.T, answer func(owner, name string, number int) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "Bearer "+fakeToken {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var body struct {
			Query     string `json:"query"`
			Variables struct {
				Owner, Name string
				Number      int
			} `json:"variables"`
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil || !strings.Contains(body.Query, "issueOrPullRequest") {
			t.Errorf("body = %s", b)
		}
		_, _ = w.Write([]byte(answer(body.Variables.Owner, body.Variables.Name, body.Variables.Number)))
	}
}

const prJSON = `{"data":{"repository":{"nameWithOwner":"org/web","issueOrPullRequest":{"__typename":"PullRequest",
"number":41,"title":"Add login form","state":"OPEN","isDraft":false,"reviewDecision":null,
"reviewRequests":{"totalCount":1},"latestOpinionatedReviews":{"nodes":[]},
"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"FAILURE"}}}]}}}}}`

func TestLookupPR(t *testing.T) {
	c, _ := newClient(t, graphQL(t, func(owner, name string, n int) string {
		if owner != "org" || name != "web" || n != 41 {
			t.Errorf("variables %s/%s#%d", owner, name, n)
		}
		return prJSON
	}))
	got, err := c.Lookup(context.Background(), fakeToken, "org/web", 41)
	if err != nil {
		t.Fatal(err)
	}
	want := itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}
	if got.Kind != itemlink.PR || got.Repo != "org/web" || got.Number != 41 || got.Values != want || got.Checks != itemlink.Failure {
		t.Errorf("Lookup = %+v", got)
	}
}

func TestLookupStatesAndReviews(t *testing.T) {
	cases := []struct {
		name, node string
		state      itemlink.State
		review     itemlink.Review
		checks     itemlink.Checks
	}{
		{
			"draft, no checks", `"state":"OPEN","isDraft":true,"reviewDecision":null,"reviewRequests":{"totalCount":0},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]}`,
			itemlink.Draft, itemlink.NoReview, itemlink.NoChecks,
		},
		{
			"merged, decision approved", `"state":"MERGED","isDraft":false,"reviewDecision":"APPROVED","reviewRequests":{"totalCount":0},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"SUCCESS"}}}]}`,
			itemlink.Merged, itemlink.Approved, itemlink.Success,
		},
		{
			"closed, latest changes requested", `"state":"CLOSED","isDraft":false,"reviewDecision":"REVIEW_REQUIRED","reviewRequests":{"totalCount":1},"latestOpinionatedReviews":{"nodes":[{"state":"APPROVED"},{"state":"CHANGES_REQUESTED"}]},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"PENDING"}}}]}`,
			itemlink.Closed, itemlink.ChangesRequested, itemlink.Running,
		},
		{
			"latest approved only", `"state":"OPEN","isDraft":false,"reviewDecision":null,"reviewRequests":{"totalCount":1},"latestOpinionatedReviews":{"nodes":[{"state":"APPROVED"}]},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"ERROR"}}}]}`,
			itemlink.Open, itemlink.Approved, itemlink.Failure,
		},
	}
	for _, cs := range cases {
		c, _ := newClient(t, graphQL(t, func(string, string, int) string {
			return `{"data":{"repository":{"nameWithOwner":"org/web","issueOrPullRequest":{"__typename":"PullRequest","number":1,"title":"t",` + cs.node + `}}}}`
		}))
		got, err := c.Lookup(context.Background(), fakeToken, "org/web", 1)
		if err != nil || got.State != cs.state || got.Review != cs.review || got.Checks != cs.checks {
			t.Errorf("%s: %+v, %v", cs.name, got, err)
		}
	}
}

func TestLookupIssue(t *testing.T) {
	c, _ := newClient(t, graphQL(t, func(string, string, int) string {
		return `{"data":{"repository":{"nameWithOwner":"org/api","issueOrPullRequest":{"__typename":"Issue","number":12,"title":"落ちる","state":"CLOSED"}}}}`
	}))
	got, err := c.Lookup(context.Background(), fakeToken, "org/api", 12)
	if err != nil || got.Kind != itemlink.Issue || got.State != itemlink.Closed || got.Review != itemlink.NoReview || got.Checks != itemlink.NoChecks {
		t.Errorf("Lookup = %+v, %v", got, err)
	}
}

func TestLookupFailures(t *testing.T) {
	cases := []struct {
		name, body string
		kind       ghcall.Kind
		number     int
	}{
		{"repository not found", `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`, ghcall.RepoUnreadable, 0},
		{"forbidden", `{"data":{"repository":null},"errors":[{"type":"FORBIDDEN","path":["repository"]}]}`, ghcall.RepoUnreadable, 0},
		{"number not found", `{"data":{"repository":{"nameWithOwner":"org/web","issueOrPullRequest":null}},"errors":[{"type":"NOT_FOUND","path":["repository","issueOrPullRequest"]}]}`, ghcall.NotFound, 7},
		{"number null without errors", `{"data":{"repository":{"nameWithOwner":"org/web","issueOrPullRequest":null}}}`, ghcall.NotFound, 7},
		{"other error", `{"errors":[{"type":"INTERNAL","message":"x"}]}`, ghcall.Unreachable, 0},
	}
	for _, cs := range cases {
		c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/org/web" { // 改名の確かめ: 改名されていない
				_, _ = w.Write([]byte(`{"full_name":"org/web"}`))
				return
			}
			_, _ = w.Write([]byte(cs.body))
		})
		_, err := c.Lookup(context.Background(), fakeToken, "org/web", 7)
		var f *ghcall.Failure
		if !ghcall.Is(err, cs.kind) || !asFailure(err, &f) || (cs.kind != ghcall.Unreachable && f.Repo != "org/web") || f.Number != cs.number {
			t.Errorf("%s: %v", cs.name, err)
		}
	}
}

func TestLookupRateLimited(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "1793000000")
		_, _ = w.Write([]byte(`{"errors":[{"type":"RATE_LIMITED","message":"x"}]}`))
	})
	_, err := c.Lookup(context.Background(), fakeToken, "org/web", 7)
	var f *ghcall.Failure
	if !asFailure(err, &f) || f.Kind != ghcall.RateLimited || f.ResetAt.Unix() != 1793000000 {
		t.Errorf("err = %v", err)
	}
}

func TestLookupFollowsRename(t *testing.T) {
	asked := []string{}
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/old/web":
			http.Redirect(w, r, "/repositories/99", http.StatusMovedPermanently)
		case "/repositories/99":
			if r.Header.Get("Authorization") != "Bearer "+fakeToken { // リダイレクト先にもトークンが届く
				http.Error(w, "no token", http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"full_name":"new-org/web2"}`))
		case "/graphql":
			graphQL(t, func(owner, name string, _ int) string {
				asked = append(asked, owner+"/"+name)
				if owner == "old" {
					return `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`
				}
				return strings.Replace(prJSON, `"org/web"`, `"new-org/web2"`, 1)
			})(w, r)
		}
	})
	got, err := c.Lookup(context.Background(), fakeToken, "old/web", 41)
	if err != nil || got.Repo != "new-org/web2" || strings.Join(asked, ",") != "old/web,new-org/web2" {
		t.Errorf("Lookup = %+v, %v, asked %v", got, err, asked)
	}
}

func TestLookupDoesNotFollowAMalformedRename(t *testing.T) {
	asked := []string{}
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/old/web":
			_, _ = w.Write([]byte(`{"full_name":"../../user"}`))
		case "/graphql":
			graphQL(t, func(owner, name string, _ int) string {
				asked = append(asked, owner+"/"+name)
				return `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`
			})(w, r)
		}
	})
	_, err := c.Lookup(context.Background(), fakeToken, "old/web", 41)
	if !ghcall.Is(err, ghcall.RepoUnreadable) || strings.Join(asked, ",") != "old/web" {
		t.Errorf("err = %v, asked %v, want only old/web", err, asked)
	}
}

func TestLookupDoesNotAskRenameOfAMalformedRepo(t *testing.T) {
	paths := []string{}
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte(`{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`))
	})
	_, err := c.Lookup(context.Background(), fakeToken, "../../user", 41)
	if !ghcall.Is(err, ghcall.RepoUnreadable) || strings.Join(paths, ",") != "/graphql" {
		t.Errorf("err = %v, paths %v, want only /graphql", err, paths)
	}
}

func TestLookupHTTPNotFoundAndForbiddenAreRepoUnreadable(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusForbidden} {
		c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/org/web" {
				_, _ = w.Write([]byte(`{"full_name":"org/web"}`))
				return
			}
			http.Error(w, "x", code)
		})
		_, err := c.Lookup(context.Background(), fakeToken, "org/web", 7)
		var f *ghcall.Failure
		if !asFailure(err, &f) || f.Kind != ghcall.RepoUnreadable || f.Repo != "org/web" {
			t.Errorf("HTTP %d: %v", code, err)
		}
	}
}

func asFailure(err error, f **ghcall.Failure) bool { return errors.As(err, f) }

// パスに整数(リストの添字)を含む入れ子のエラーがあっても、node があれば読める。
func TestLookupNestedErrorWithIntegerPath(t *testing.T) {
	c, _ := newClient(t, graphQL(t, func(string, string, int) string {
		return strings.TrimSuffix(prJSON, "}") + `,"errors":[{"type":"INTERNAL","path":["repository","issueOrPullRequest","commits","nodes",0,"commit"]}]}`
	}))
	got, err := c.Lookup(context.Background(), fakeToken, "org/web", 41)
	if err != nil || got.Number != 41 {
		t.Errorf("Lookup = %+v, %v", got, err)
	}
}
