package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// OnCode is the code in a type of usecase (the GUI's server does not import
// lib/github), without the device code that only the polling uses; ShowCode
// still gets the whole of it.
func TestAuthLoginDeviceOnCode(t *testing.T) {
	t.Parallel()
	ts, _ := authSession(t, fakegh.State{AutoApprove: "alice"}, "")
	var shown *github.DeviceCode
	var got DeviceCode
	res, err := ts.AuthLogin(AuthLoginInput{
		Host:     "github.com",
		ShowCode: func(dc *github.DeviceCode) { shown = dc },
		OnCode:   func(c DeviceCode) { got = c },
	})
	if err != nil || !res.Added {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if shown == nil || got.UserCode != shown.UserCode || got.VerificationURI != shown.VerificationURI || got.ExpiresIn != 900 {
		t.Errorf("OnCode got %+v; ShowCode got %+v", got, shown)
	}
}

// The context ends the polling with auth.canceled, and nothing is written.
func TestAuthLoginDeviceCancel(t *testing.T) {
	t.Parallel()
	five := 5 // seconds: no poll would be answered in time
	ts, k := authSession(t, fakegh.State{DeviceInterval: &five}, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got DeviceCode
	start := time.Now()
	_, err := ts.AuthLogin(AuthLoginInput{
		Host: "github.com", Context: ctx,
		OnCode: func(c DeviceCode) { got = c; cancel() },
	})
	wantErr(t, err, KindInvalid, "auth.canceled")
	if got.UserCode == "" || got.VerificationURI == "" {
		t.Errorf("code = %+v", got)
	}
	if len(k.m) != 0 || len(ts.accounts()) != 0 {
		t.Errorf("keychain %v, accounts %v; want nothing written", k.m, ts.accounts())
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("the cancel took %v", time.Since(start))
	}
}

// The errors the GUI shows have a Code (they had none, so the server said
// internal); their messages are what the CLI printed before.
func TestAuthLoginErrorCodes(t *testing.T) {
	t.Parallel()
	t.Run("no client id", func(t *testing.T) {
		t.Parallel()
		ts := newTestSession(t)
		ts.Keyring = newKR()
		ts.setConfig("", cfgStore{"me", ""})
		_, err := ts.AuthLogin(AuthLoginInput{Host: "github.com", OnCode: func(DeviceCode) { t.Error("the code was shown") }})
		wantErr(t, err, KindInvalid, "auth.client_id")
		if AsError(err).Args["host"] != "github.com" || !strings.Contains(err.Error(), "the device flow needs hosts[].client_id for github.com in config.yaml") {
			t.Errorf("err = %v, args %v", err, AsError(err).Args)
		}
	})
	t.Run("the token is of another login", func(t *testing.T) {
		t.Parallel()
		ts, k := authSession(t, fakegh.State{Tokens: map[string]string{"t-bob": "bob"}},
			"accounts:\n  - {name: work, host: github.com, login: alice}\n")
		_, err := ts.AuthLogin(AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
		wantErr(t, err, KindInvalid, "auth.mismatch")
		a := AsError(err).Args
		if a["account"] != "work" || a["host"] != "github.com" || a["login"] != "bob" {
			t.Errorf("args = %v", a)
		}
		if len(k.m) != 0 {
			t.Errorf("keychain = %v", k.m)
		}
	})
	t.Run("empty token", func(t *testing.T) {
		t.Parallel()
		ts, _ := authSession(t, fakegh.State{}, "")
		_, err := ts.AuthLogin(AuthLoginInput{Host: "github.com", WithToken: true, Token: " \n"})
		wantErr(t, err, KindInvalid, "auth.token_empty")
	})
}
