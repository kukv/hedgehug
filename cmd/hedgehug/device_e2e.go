//go:build e2e

package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// e2eGitHub・e2eDevice・e2eTokens は、wire が口に差し込み、wrap の入口が答えを書き込む。同じものを使うので、
// パッケージに 1 つずつ置く(D2 画面の計画の決定 S21・S22)。
var (
	e2eTokens, e2eUsers = e2eLogins()
	e2eGitHub           = &apptest.GitHub{Users: e2eUsers}
	e2eDeviceFlow       = newE2EDevice(e2eGitHub)
)

// e2eLogins は、gh CLI などのトークンで確かめられるログイン名とトークン。taro と、
// 3 つの幅 × やり直し 2 回 × 2 人(gh-{幅}-{回数}-{a か b})。
func e2eLogins() (tokens, users map[string]string) {
	tokens = map[string]string{"taro": "e2e-gh-token"}
	users = map[string]string{"e2e-gh-token": "taro"}
	for _, width := range []string{"phone", "tablet", "desktop"} {
		for _, retry := range []string{"0", "1"} {
			for _, who := range []string{"a", "b"} {
				login := "gh-" + width + "-" + retry + "-" + who
				tokens[login] = "tok-" + login
				users["tok-"+login] = login
			}
		}
	}
	return tokens, users
}

// e2eDevice は、通しのテストで GitHub のデバイスフローの代わりをする。コードは始めるたびに 0000-0001 から順に振り、
// テストが answer(POST /e2e/device)で、そのコードの答えを決める。答えがなければ待っている。
type e2eDevice struct {
	mu      sync.Mutex
	gh      *apptest.GitHub
	started int
	devices map[string]string           // user_code → device_code
	answers map[string]port.DeviceGrant // device_code → 答え
}

func newE2EDevice(gh *apptest.GitHub) *e2eDevice {
	return &e2eDevice{gh: gh, devices: map[string]string{}, answers: map[string]port.DeviceGrant{}}
}

func (d *e2eDevice) StartDevice(context.Context, string) (port.DeviceCode, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.started++
	dc := fmt.Sprintf("device-%d", d.started)
	uc := fmt.Sprintf("%04d-%04d", d.started/10000, d.started%10000)
	d.devices[uc] = dc
	return port.DeviceCode{
		DeviceCode: dc, UserCode: uc, VerificationURI: "https://github.com/login/device",
		ExpiresIn: 15 * time.Minute, Interval: time.Second,
	}, nil
}

func (d *e2eDevice) PollDevice(_ context.Context, _, deviceCode string) (port.DeviceGrant, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if g, ok := d.answers[deviceCode]; ok {
		return g, nil
	}
	return port.DeviceGrant{Status: port.DevicePending}, nil
}

func (d *e2eDevice) RefreshDevice(context.Context, string, string) (account.DeviceToken, error) {
	return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
}

// answer は、userCode のコードの答えを決める。grant は、login の人が許可したことにする(トークンを GitHub の偽物に足す)。
func (d *e2eDevice) answer(userCode, result, login string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	dc, ok := d.devices[userCode]
	if !ok {
		return fmt.Errorf("unknown code %q", userCode)
	}
	switch result {
	case "grant":
		tok := "tok-" + dc
		d.gh.AddUser(tok, login)
		d.answers[dc] = port.DeviceGrant{Status: port.DeviceGranted, Token: account.DeviceToken{Access: tok}}
	case "deny":
		d.answers[dc] = port.DeviceGrant{Status: port.DeviceDenied}
	case "expire":
		d.answers[dc] = port.DeviceGrant{Status: port.DeviceExpired}
	default:
		return errors.New("result is grant, deny or expire")
	}
	return nil
}

// handler は、POST /e2e/device?code=…&result=…&login=… を受け、ほかの要求を next に渡す。API と同じトークンを求める。
func (d *e2eDevice) handler(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/device" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		if err := d.answer(q.Get("code"), q.Get("result"), q.Get("login")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// wrap は、通しのテストがデバイスフローの答えを決める入口を足す。
func wrap(token string, h http.Handler) http.Handler { return e2eDeviceFlow.handler(token, h) }
