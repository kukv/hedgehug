package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// Viewer は、token の持ち主のログイン名を返す(GET /user)。
func (c *Client) Viewer(ctx context.Context, token string) (string, error) {
	var u struct {
		Login string `json:"login"`
	}
	if err := c.get(ctx, token, "/user", &u); err != nil {
		return "", err
	}
	if u.Login == "" {
		// 応答が読めなかったのと同じに扱う(「このトークンは @ のものです」と出さないため)。
		return "", &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return u.Login, nil
}
