package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// gqlError は、GraphQL の答えの errors の 1 件。
type gqlError struct {
	Type    string `json:"type"`
	Path    []any  `json:"path"`
	Message string `json:"message"`
}

// gqlRate は、答えの rateLimit。
type gqlRate struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

// port は port.Rate にする。答えになければ(nil)ゼロ値。
func (r *gqlRate) port() port.Rate {
	if r == nil {
		return port.Rate{}
	}
	return port.Rate{Limit: r.Limit, Remaining: r.Remaining, ResetAt: r.ResetAt}
}

// graphql は、body を GraphQL に送って out に読む。HTTP の 403・404 はリポジトリを読めない失敗にする。
func (c *Client) graphql(ctx context.Context, token, repo string, body any, out any) (http.Header, error) {
	b, _ := json.Marshal(body)
	h, err := c.do(ctx, http.MethodPost, token, "/graphql", bytes.NewReader(b), out)
	if err != nil {
		return h, withRepo(err, repo)
	}
	return h, nil
}

// gqlFailure は、ReadMany・ReadPulls の答えの失敗(D3-5)。repoMissing は data.repository が null か。
// App 未導入の判定(hiddenByApp)をここ 1 か所にそろえる。
func gqlFailure(errs []gqlError, repoMissing bool, repo string, reset time.Time) *ghcall.Failure {
	for _, e := range errs {
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
	case repoMissing && len(errs) == 0:
		return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
	case repoMissing:
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return nil
}
