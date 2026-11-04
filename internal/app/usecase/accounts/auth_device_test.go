package accounts_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// OnCode is the code in a type of usecase (the GUI's server does not import
// lib/github), without the device code that only the polling uses.
func TestAuthLoginDeviceOnCode(t *testing.T) {
	t.Parallel()
	ts, _ := usecasetest.AuthSession(t, fakegh.State{AutoApprove: "alice"}, "")
	var got accounts.DeviceCode
	res, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{
		Host:   "github.com",
		OnCode: func(c accounts.DeviceCode) { got = c },
	})
	if err != nil || !res.Added {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if got.UserCode == "" || got.VerificationURI == "" || got.ExpiresIn != 900 {
		t.Errorf("OnCode got %+v", got)
	}
}

// The context ends the polling with auth.canceled, and nothing is written.
func TestAuthLoginDeviceCancel(t *testing.T) {
	t.Parallel()
	five := 5 // seconds: no poll would be answered in time
	ts, k := usecasetest.AuthSession(t, fakegh.State{DeviceInterval: &five}, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got accounts.DeviceCode
	start := time.Now()
	_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{
		Host: "github.com", Context: ctx,
		OnCode: func(c accounts.DeviceCode) { got = c; cancel() },
	})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "auth.canceled")
	if got.UserCode == "" || got.VerificationURI == "" {
		t.Errorf("code = %+v", got)
	}
	if len(k.M) != 0 || len(ts.Accounts()) != 0 {
		t.Errorf("keychain %v, accounts %v; want nothing written", k.M, ts.Accounts())
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
		ts := usecasetest.New(t)
		ts.Keyring = usecasetest.NewKeyring()
		ts.SetConfig("", usecasetest.CfgStore{Name: "me", Account: ""})
		_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com", OnCode: func(accounts.DeviceCode) { t.Error("the code was shown") }})
		usecasetest.WantErr(t, err, usecase.KindInvalid, "auth.client_id")
		if usecase.AsError(err).Args["host"] != "github.com" || !strings.Contains(err.Error(), "the device flow needs hosts[].client_id for github.com in config.yaml") {
			t.Errorf("err = %v, args %v", err, usecase.AsError(err).Args)
		}
	})
	t.Run("the token is of another login", func(t *testing.T) {
		t.Parallel()
		ts, k := usecasetest.AuthSession(t, fakegh.State{Tokens: map[string]string{"t-bob": "bob"}},
			"accounts:\n  - {name: work, host: github.com, login: alice}\n")
		_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
		usecasetest.WantErr(t, err, usecase.KindInvalid, "auth.mismatch")
		a := usecase.AsError(err).Args
		if a["account"] != "work" || a["host"] != "github.com" || a["login"] != "bob" {
			t.Errorf("args = %v", a)
		}
		if len(k.M) != 0 {
			t.Errorf("keychain = %v", k.M)
		}
	})
	t.Run("empty token", func(t *testing.T) {
		t.Parallel()
		ts, _ := usecasetest.AuthSession(t, fakegh.State{}, "")
		_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com", WithToken: true, Token: " \n"})
		usecasetest.WantErr(t, err, usecase.KindInvalid, "auth.token_empty")
	})
}
