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
	"github.com/kukv/hedgehug/internal/domain/prlink"
)

// pullsQuery は、PR を更新の新しい順に 100 件ずつ読む GraphQL(D4b)。コメントは古い順に 100 件まで。
const pullsQuery = `query Pulls($owner: String!, $name: String!, $after: String) {
rateLimit { limit remaining resetAt }
repository(owner: $owner, name: $name) { nameWithOwner
pullRequests(first: 100, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
pageInfo { hasNextPage endCursor }
nodes { ...F headRefName body updatedAt comments(first: 100) { nodes { body } } } } } }
` + nodeFragment

type pullNode struct {
	node
	HeadRefName string    `json:"headRefName"`
	Body        string    `json:"body"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Comments    struct {
		Nodes []struct {
			Body string `json:"body"`
		} `json:"nodes"`
	} `json:"comments"`
}

type pullsAnswer struct {
	Data struct {
		RateLimit *struct {
			Limit     int       `json:"limit"`
			Remaining int       `json:"remaining"`
			ResetAt   time.Time `json:"resetAt"`
		} `json:"rateLimit"`
		Repository *struct {
			NameWithOwner string `json:"nameWithOwner"`
			PullRequests  struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []pullNode `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Type    string `json:"type"`
		Path    []any  `json:"path"`
		Message string `json:"message"`
	} `json:"errors"`
}

// ReadPulls は port.GitHub.ReadPulls。リポジトリを読めなければ、Lookup と同じに改名・移管を追い、App 未導入を確かめる(attempt)。
func (c *Client) ReadPulls(ctx context.Context, token, repo string, since time.Time) (port.Pulls, error) {
	return attempt(ctx, c, token, repo, func(repo string) (port.Pulls, error) {
		return c.readPullsOnce(ctx, token, repo, since)
	})
}

// readPullsOnce は、since がゼロなら最初のページだけ、そうでなければ since より古い PR に当たるまでページをめくる。
func (c *Client) readPullsOnce(ctx context.Context, token, repo string, since time.Time) (port.Pulls, error) {
	out := port.Pulls{Repo: repo}
	after := ""
	for {
		a, err := c.pullsPage(ctx, token, repo, after)
		if err != nil {
			return port.Pulls{}, err
		}
		r := a.Data.Repository
		out.Repo = r.NameWithOwner
		if rl := a.Data.RateLimit; rl != nil {
			out.Rate = port.Rate{Limit: rl.Limit, Remaining: rl.Remaining, ResetAt: rl.ResetAt}
		}
		reached := false
		for i := range r.PullRequests.Nodes {
			n := &r.PullRequests.Nodes[i]
			if !since.IsZero() && n.UpdatedAt.Before(since) {
				reached = true
				break
			}
			out.Pulls = append(out.Pulls, n.pull())
		}
		pi := r.PullRequests.PageInfo
		if since.IsZero() || reached || !pi.HasNextPage {
			return out, nil
		}
		after = pi.EndCursor
	}
}

func (c *Client) pullsPage(ctx context.Context, token, repo, after string) (pullsAnswer, error) {
	owner, name, _ := strings.Cut(repo, "/")
	vars := map[string]any{"owner": owner, "name": name}
	if after != "" {
		vars["after"] = after
	}
	body, _ := json.Marshal(map[string]any{"query": pullsQuery, "variables": vars})
	var a pullsAnswer
	h, err := c.do(ctx, http.MethodPost, token, "/graphql", bytes.NewReader(body), &a)
	if err != nil {
		return a, withRepo(err, repo)
	}
	if f := a.failure(repo, c.resetAt(h)); f != nil {
		return a, f
	}
	return a, nil
}

func (n *pullNode) pull() port.Pull {
	f := found("", &n.node)
	p := port.Pull{
		Number: n.Number, Values: f.Values, Checks: f.Checks, UpdatedAt: n.UpdatedAt,
		Text: prlink.Text{Branch: n.HeadRefName, Title: n.Title, Body: n.Body},
	}
	for _, c := range n.Comments.Nodes {
		p.Comments = append(p.Comments, c.Body)
	}
	return p
}

// failure は、答えの失敗(D3-5)。PR の一覧には番号がないので、NotFound は出さない。
func (a pullsAnswer) failure(repo string, reset time.Time) *ghcall.Failure {
	for _, e := range a.Errors {
		switch {
		case e.Type == "RATE_LIMITED":
			return &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: reset}
		case hiddenByApp(e.Type, e.Path, e.Message):
			return appMissing(repo)
		case (e.Type == "NOT_FOUND" || e.Type == "FORBIDDEN") && len(e.Path) == 1:
			return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
		}
	}
	switch {
	case a.Data.Repository == nil && len(a.Errors) == 0:
		return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
	case a.Data.Repository == nil:
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return nil
}
