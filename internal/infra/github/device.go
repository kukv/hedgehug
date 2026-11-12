package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// tokenAnswer は /login/oauth/access_token の答え。失敗は 200 の本文の error で返る。
type tokenAnswer struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
}

// StartDevice は、デバイスフローを始める(POST /login/device/code)。
func (c *Client) StartDevice(ctx context.Context, clientID string) (port.DeviceCode, error) {
	var a struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
	}
	if err := c.postForm(ctx, "/login/device/code", url.Values{"client_id": {clientID}}, &a, pollFailure); err != nil {
		return port.DeviceCode{}, err
	}
	if a.Error != "" || a.DeviceCode == "" {
		return port.DeviceCode{}, &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return port.DeviceCode{
		DeviceCode: a.DeviceCode, UserCode: a.UserCode, VerificationURI: a.VerificationURI,
		ExpiresIn: time.Duration(a.ExpiresIn) * time.Second, Interval: time.Duration(a.Interval) * time.Second,
	}, nil
}

// PollDevice は、許可されたかを 1 回だけ聞く。
func (c *Client) PollDevice(ctx context.Context, clientID, deviceCode string) (port.DeviceGrant, error) {
	var a tokenAnswer
	err := c.postForm(ctx, "/login/oauth/access_token", url.Values{
		"client_id": {clientID}, "device_code": {deviceCode},
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
	}, &a, pollFailure)
	if err != nil {
		return port.DeviceGrant{}, err
	}
	switch a.Error {
	case "":
		if a.AccessToken == "" {
			return port.DeviceGrant{}, &ghcall.Failure{Kind: ghcall.Unreachable}
		}
		return port.DeviceGrant{Status: port.DeviceGranted, Token: c.tokenOf(a)}, nil
	case "authorization_pending":
		return port.DeviceGrant{Status: port.DevicePending}, nil
	case "slow_down":
		return port.DeviceGrant{Status: port.DeviceSlowDown}, nil
	case "access_denied":
		return port.DeviceGrant{Status: port.DeviceDenied}, nil
	}
	// expired_token と、コードがもう使えないほかの答え(incorrect_device_code など)。再試行で新しいコードを出す。
	return port.DeviceGrant{Status: port.DeviceExpired}, nil
}

// RefreshDevice は、更新用のトークンで取り直す。
func (c *Client) RefreshDevice(ctx context.Context, clientID, refresh string) (account.DeviceToken, error) {
	var a tokenAnswer
	err := c.postForm(ctx, "/login/oauth/access_token", url.Values{
		"client_id": {clientID}, "refresh_token": {refresh}, "grant_type": {"refresh_token"},
	}, &a, refreshFailure)
	if err != nil {
		return account.DeviceToken{}, err
	}
	if a.Error != "" || a.AccessToken == "" {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return c.tokenOf(a), nil
}

// tokenOf は、答えのトークンの組。expires_in が 0 なら期限なし。
func (c *Client) tokenOf(a tokenAnswer) account.DeviceToken {
	now := c.Now()
	t := account.DeviceToken{Access: a.AccessToken, Refresh: a.RefreshToken}
	if a.ExpiresIn > 0 {
		t.ExpiresAt = now.Add(time.Duration(a.ExpiresIn) * time.Second)
	}
	if a.RefreshTokenExpiresIn > 0 {
		t.RefreshExpiresAt = now.Add(time.Duration(a.RefreshTokenExpiresIn) * time.Second)
	}
	return t
}

// pollFailure は、StartDevice・PollDevice で 200 以外が返ったときの種類。
// 利用上限のほかは宛先の間違いなどなので、「見つかりません」や「トークンが受け付けられません」にせず「通信できない」にする。
func pollFailure(_ int, limited *ghcall.Failure) *ghcall.Failure {
	if limited != nil {
		return limited
	}
	return &ghcall.Failure{Kind: ghcall.Unreachable}
}

// refreshFailure は、RefreshDevice で 200 以外が返ったときの種類。
// 利用上限は拒否にしない(先に判定する)。401・403・404 は更新用のトークンの拒否。
func refreshFailure(status int, limited *ghcall.Failure) *ghcall.Failure {
	if limited != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return &ghcall.Failure{Kind: ghcall.Unreachable}
}

// postForm は、WebURL+path にフォームを POST し、2xx なら本文を out に読む。
// 2xx でなければ、状態コードと(利用上限なら)その失敗を onFail に渡し、返った失敗を返す。失敗は *ghcall.Failure。
func (c *Client) postForm(ctx context.Context, path string, form url.Values, out any,
	onFail func(status int, limited *ghcall.Failure) *ghcall.Failure,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.WebURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		// net/http のエラーは URL を含むことがあるので、種類だけを返す。
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return onFail(res.StatusCode, c.rateLimited(res))
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out); err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return nil
}
