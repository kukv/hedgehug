package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// nodeFragment は、PR・Issue から読むフィールド(Lookup と ReadMany で共通)。レビューは reviewDecision と各レビュアーの
// 最新の意見から、チェックは最後のコミットの statusCheckRollup から決める(前回の実装と同じ)。
const nodeFragment = `fragment F on IssueOrPullRequest {
__typename
... on Issue { number title state }
... on PullRequest { number title state isDraft reviewDecision
reviewRequests(first: 1) { totalCount }
latestOpinionatedReviews(first: 100) { nodes { state } }
commits(last: 1) { nodes { commit { statusCheckRollup { state } } } } }
}`

// lookupQuery は、1 つの番号の PR か Issue を読む GraphQL(D3-4)。
const lookupQuery = `query Lookup($owner: String!, $name: String!, $number: Int!) {
repository(owner: $owner, name: $name) { nameWithOwner issueOrPullRequest(number: $number) { ...F } } }
` + nodeFragment

type lookupAnswer struct {
	Data struct {
		Repository *struct {
			NameWithOwner      string `json:"nameWithOwner"`
			IssueOrPullRequest *node  `json:"issueOrPullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Type string `json:"type"`
		Path []any  `json:"path"`
	} `json:"errors"`
}

type node struct {
	Typename       string  `json:"__typename"`
	Number         int     `json:"number"`
	Title          string  `json:"title"`
	State          string  `json:"state"`
	IsDraft        bool    `json:"isDraft"`
	ReviewDecision *string `json:"reviewDecision"`
	ReviewRequests struct {
		TotalCount int `json:"totalCount"`
	} `json:"reviewRequests"`
	LatestOpinionatedReviews struct {
		Nodes []struct {
			State string `json:"state"`
		} `json:"nodes"`
	} `json:"latestOpinionatedReviews"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

// Lookup は port.GitHub.Lookup。リポジトリが見つからなければ、REST で改名・移管を確かめ、名前が変わっていれば
// 新しい名前で 1 回だけ問い合わせ直す。
func (c *Client) Lookup(ctx context.Context, token, repo string, number int) (port.Found, error) {
	f, err := c.lookupOnce(ctx, token, repo, number)
	if !ghcall.Is(err, ghcall.RepoUnreadable) {
		return f, err
	}
	moved, ok := c.renamed(ctx, token, repo)
	if !ok {
		return f, err
	}
	return c.lookupOnce(ctx, token, moved, number)
}

func (c *Client) lookupOnce(ctx context.Context, token, repo string, number int) (port.Found, error) {
	owner, name, _ := strings.Cut(repo, "/")
	body, _ := json.Marshal(map[string]any{
		"query": lookupQuery, "variables": map[string]any{"owner": owner, "name": name, "number": number},
	})
	var a lookupAnswer
	h, err := c.do(ctx, http.MethodPost, token, "/graphql", bytes.NewReader(body), &a)
	if err != nil {
		return port.Found{}, withRepo(err, repo)
	}
	if f := answerFailure(a, repo, number, c.resetAt(h)); f != nil {
		return port.Found{}, f
	}
	n := a.Data.Repository.IssueOrPullRequest
	return found(a.Data.Repository.NameWithOwner, n), nil
}

// withRepo は、Forbidden・NotFound(HTTP の 403・404)をリポジトリを読めない失敗にする。
func withRepo(err error, repo string) error {
	if ghcall.Is(err, ghcall.Forbidden) || ghcall.Is(err, ghcall.NotFound) {
		return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
	}
	return err
}

// answerFailure は、GraphQL の答えの失敗(D3-5)。失敗でなければ nil。
func answerFailure(a lookupAnswer, repo string, number int, reset time.Time) *ghcall.Failure {
	for _, e := range a.Errors {
		switch {
		case e.Type == "RATE_LIMITED":
			return &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: reset}
		case (e.Type == "NOT_FOUND" || e.Type == "FORBIDDEN") && len(e.Path) == 1:
			return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
		case e.Type == "NOT_FOUND" && len(e.Path) == 2:
			return &ghcall.Failure{Kind: ghcall.NotFound, Repo: repo, Number: number}
		}
	}
	switch {
	case a.Data.Repository == nil && len(a.Errors) == 0:
		return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
	case a.Data.Repository == nil:
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	case a.Data.Repository.IssueOrPullRequest == nil:
		return &ghcall.Failure{Kind: ghcall.NotFound, Repo: repo, Number: number}
	}
	return nil
}

// renamed は、repo が改名・移管されていれば、いまの名前を返す(REST はリダイレクトで移動先を返す)。
// repo が owner/name の形でなければ問い合わせない。いまの名前が owner/name の形でなければ、改名とみなさない。
func (c *Client) renamed(ctx context.Context, token, repo string) (string, bool) {
	if !wellFormed(repo) {
		return "", false
	}
	var r struct {
		FullName string `json:"full_name"`
	}
	if err := c.get(ctx, token, "/repos/"+repo, &r); err != nil || r.FullName == "" || strings.EqualFold(r.FullName, repo) {
		return "", false
	}
	if !wellFormed(r.FullName) {
		return "", false
	}
	return r.FullName, true
}

// wellFormed は、repo が前後の空白なしで owner/name の形か。
func wellFormed(repo string) bool {
	r, err := project.NormalizeRepo(repo)
	return err == nil && r == repo
}

func found(repo string, n *node) port.Found {
	f := port.Found{Repo: repo, Number: n.Number}
	f.Title = n.Title
	if n.Typename == "Issue" {
		f.Kind, f.Review = itemlink.Issue, itemlink.NoReview
		f.State = itemlink.Open
		if n.State == "CLOSED" {
			f.State = itemlink.Closed
		}
		return f
	}
	f.Kind = itemlink.PR
	switch {
	case n.State == "MERGED":
		f.State = itemlink.Merged
	case n.State == "CLOSED":
		f.State = itemlink.Closed
	case n.IsDraft:
		f.State = itemlink.Draft
	default:
		f.State = itemlink.Open
	}
	f.Review, f.Checks = n.review(), n.checks()
	return f
}

// review は 07「表示と更新」のレビューの状態の決め方。
func (n *node) review() itemlink.Review {
	if d := n.ReviewDecision; d != nil {
		switch *d {
		case "APPROVED":
			return itemlink.Approved
		case "CHANGES_REQUESTED":
			return itemlink.ChangesRequested
		}
	}
	approved := false
	for _, r := range n.LatestOpinionatedReviews.Nodes {
		switch r.State {
		case "CHANGES_REQUESTED":
			return itemlink.ChangesRequested
		case "APPROVED":
			approved = true
		}
	}
	switch {
	case approved:
		return itemlink.Approved
	case n.ReviewRequests.TotalCount > 0:
		return itemlink.Pending
	}
	return itemlink.NoReview
}

// checks は D3-6 のチェックの結果。
func (n *node) checks() itemlink.Checks {
	if len(n.Commits.Nodes) == 0 || n.Commits.Nodes[0].Commit.StatusCheckRollup == nil {
		return itemlink.NoChecks
	}
	switch n.Commits.Nodes[0].Commit.StatusCheckRollup.State {
	case "SUCCESS":
		return itemlink.Success
	case "FAILURE", "ERROR":
		return itemlink.Failure
	}
	return itemlink.Running
}
