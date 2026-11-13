// Package github は、GitHub の REST API への問い合わせ(port.GitHub の実装)を持つ。
// トークンは Authorization の見出しにだけ載せ、エラーの文には書かない。
package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

const (
	apiURL     = "https://api.github.com"
	webURL     = "https://github.com"
	apiVersion = "2022-11-28"
	// timeout は 1 回の問い合わせの打ち切り(07「GitHub との通信に失敗したとき」)。
	timeout = 30 * time.Second
	// maxBody は、読む応答の大きさの上限。
	maxBody = 10 << 20
)

// Client は GitHub の API に問い合わせる。
type Client struct {
	BaseURL string
	WebURL  string // デバイスフローの宛先(github.com)
	HTTP    *http.Client
	Now     func() time.Time
	// OnSkew は、応答の Date からこのマシンの時計を引いたずれを、応答のたびに受け取る(トラック C が記録する)。
	OnSkew func(time.Duration)
}

func New(onSkew func(time.Duration)) *Client {
	return &Client{BaseURL: apiURL, WebURL: webURL, HTTP: &http.Client{Timeout: timeout}, Now: time.Now, OnSkew: onSkew}
}

// get は、path を GET して、成功なら本文を out に読む。失敗は *ghcall.Failure。
func (c *Client) get(ctx context.Context, token, path string, out any) error {
	_, err := c.do(ctx, http.MethodGet, token, path, nil, out)
	return err
}

// do は、問い合わせを 1 回行う。成功なら本文を out に読み、応答の見出しを返す(GraphQL の利用上限の時刻に使う)。
// GET のリダイレクトは net/http が同じホストの中で追う(改名・移管の追従。D3-4)。
func (c *Client) do(ctx context.Context, method, token, path string, body io.Reader, out any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		// net/http のエラーは URL を含むことがあるので、種類だけを返す。
		return nil, &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	defer func() { _ = res.Body.Close() }()
	c.observeSkew(res.Header.Get("Date"))
	if f := c.classify(res); f != nil {
		return res.Header, f
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out); err != nil {
		return res.Header, &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return res.Header, nil
}

// classify は、成功でない応答を失敗の種類に分ける。成功なら nil。
func (c *Client) classify(res *http.Response) *ghcall.Failure {
	code := res.StatusCode
	limit := c.rateLimited(res)
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusUnauthorized:
		return &ghcall.Failure{Kind: ghcall.TokenRejected}
	case limit != nil:
		return limit
	case code == http.StatusForbidden:
		return &ghcall.Failure{Kind: ghcall.Forbidden}
	case code == http.StatusNotFound:
		return &ghcall.Failure{Kind: ghcall.NotFound}
	}
	return &ghcall.Failure{Kind: ghcall.Unreachable}
}

// rateLimited は、利用上限の応答(429、または上限の見出しが付いた 403)なら RateLimited の失敗を、そうでなければ nil を返す。
func (c *Client) rateLimited(res *http.Response) *ghcall.Failure {
	code := res.StatusCode
	if code == http.StatusTooManyRequests ||
		(code == http.StatusForbidden && (res.Header.Get("X-RateLimit-Remaining") == "0" || res.Header.Get("Retry-After") != "")) {
		return &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: c.resetAt(res.Header)}
	}
	return nil
}

// resetAt は、利用上限が戻る時刻。秒数(Retry-After)があれば今に足し、なければ X-RateLimit-Reset の時刻。
// どちらも分からなければゼロ(07「GitHub との通信に失敗したとき」)。
func (c *Client) resetAt(h http.Header) time.Time {
	if n, err := strconv.Atoi(h.Get("Retry-After")); err == nil {
		return c.Now().Add(time.Duration(n) * time.Second)
	}
	if n, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		return time.Unix(n, 0)
	}
	return time.Time{}
}

func (c *Client) observeSkew(date string) {
	if c.OnSkew == nil || date == "" {
		return
	}
	t, err := http.ParseTime(date)
	if err != nil {
		return
	}
	c.OnSkew(t.Sub(c.Now()))
}
