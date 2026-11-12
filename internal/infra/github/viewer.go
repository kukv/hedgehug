package github

import "context"

// Viewer は、token の持ち主のログイン名を返す(GET /user)。
func (c *Client) Viewer(ctx context.Context, token string) (string, error) {
	var u struct {
		Login string `json:"login"`
	}
	if err := c.get(ctx, token, "/user", &u); err != nil {
		return "", err
	}
	return u.Login, nil
}
