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
	apiVersion = "2022-11-28"
	// timeout は 1 回の問い合わせの打ち切り(07「GitHub との通信に失敗したとき」)。
	timeout = 30 * time.Second
	// maxBody は、読む応答の大きさの上限。
	maxBody = 10 << 20
)

// Client は GitHub の API に問い合わせる。
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Now     func() time.Time
	// OnSkew は、応答の Date からこのマシンの時計を引いたずれを、応答のたびに受け取る(トラック C が記録する)。
	OnSkew func(time.Duration)
}

func New(onSkew func(time.Duration)) *Client {
	return &Client{BaseURL: apiURL, HTTP: &http.Client{Timeout: timeout}, Now: time.Now, OnSkew: onSkew}
}

// get は、path を GET して、成功なら本文を out に読む。失敗は *ghcall.Failure。
func (c *Client) get(ctx context.Context, token, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	res, err := c.HTTP.Do(req)
	if err != nil {
		// net/http のエラーは URL を含むことがあるので、種類だけを返す。
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	defer func() { _ = res.Body.Close() }()
	c.observeSkew(res.Header.Get("Date"))
	if f := c.classify(res); f != nil {
		return f
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out); err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return nil
}

// classify は、成功でない応答を失敗の種類に分ける。成功なら nil。
func (c *Client) classify(res *http.Response) *ghcall.Failure {
	code := res.StatusCode
	limited := code == http.StatusTooManyRequests ||
		(code == http.StatusForbidden && (res.Header.Get("X-RateLimit-Remaining") == "0" || res.Header.Get("Retry-After") != ""))
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusUnauthorized:
		return &ghcall.Failure{Kind: ghcall.TokenRejected}
	case limited:
		return &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: c.resetAt(res.Header)}
	case code == http.StatusForbidden:
		return &ghcall.Failure{Kind: ghcall.Forbidden}
	case code == http.StatusNotFound:
		return &ghcall.Failure{Kind: ghcall.NotFound}
	}
	return &ghcall.Failure{Kind: ghcall.Unreachable}
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
