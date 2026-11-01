package ops

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/fakegh"
)

// A token that cannot be got (env unset) is auth.token, through Token, GitHub
// and Conn.Auth.
func TestTokenFailureIsAuthToken(t *testing.T) {
	ts, _ := authSession(t, fakegh.State{},
		"accounts:\n  - {name: e, host: github.com, login: alice, token: \"env:HH_UNSET_TOKEN\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	acct, ok := c.Account("e")
	if !ok {
		t.Fatal("no account e")
	}
	_, err = ts.Token(c, acct)
	wantErr(t, err, KindInvalid, "auth.token")
	if got := AsError(err).Args["account"]; got != "e" {
		t.Errorf("Args[account] = %v", got)
	}
	_, err = ts.GitHub(c, acct, "github.com")
	wantErr(t, err, KindInvalid, "auth.token")

	auth := ts.Conn(c, c.Stores[0]).Auth
	if auth == nil {
		t.Fatal("no Conn.Auth")
	}
	_, err = auth("https://github.com/alice/x.git")
	wantErr(t, err, KindInvalid, "auth.token")
}

// A token GitHub refuses (401) when AuthLogin checks it is auth.unauthorized.
func TestAuthLoginBadTokenIsUnauthorized(t *testing.T) {
	ts, k := authSession(t, fakegh.State{}, "")
	_, err := ts.AuthLogin(AuthLoginInput{Host: "github.com", WithToken: true, Token: "bad"})
	wantErr(t, err, KindInvalid, "auth.unauthorized")
	for _, want := range []string{"github.com refused the token", "has not expired"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("text = %v; want %q", err, want)
		}
	}
	for _, not := range []string{"(new account)", "auth login", "cannot check"} {
		if strings.Contains(err.Error(), not) {
			t.Errorf("text = %v; must not contain %q", err, not)
		}
	}
	if len(k.m) != 0 || len(ts.accounts()) != 0 {
		t.Errorf("keychain %v, accounts %v", k.m, ts.accounts())
	}
}

// The login of the token is not the account's: the message says the token
// can be revoked on GitHub.
func TestAuthLoginOtherLoginSaysHowToRevoke(t *testing.T) {
	ts, _ := authSession(t, fakegh.State{Tokens: map[string]string{"t-bob": "bob"}},
		"accounts:\n  - {name: work, host: github.com, login: alice}\n")
	_, err := ts.AuthLogin(AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
	if err == nil || !strings.Contains(err.Error(), "Authorized GitHub Apps") {
		t.Errorf("err = %v", err)
	}
}

// For a known account too: the same text, with no hint to run the command.
func TestAuthLoginBadTokenKnownAccount(t *testing.T) {
	ts, _ := authSession(t, fakegh.State{}, "accounts:\n  - {name: work, host: github.com, login: alice, token: keychain}\n")
	_, err := ts.AuthLogin(AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "bad"})
	wantErr(t, err, KindInvalid, "auth.unauthorized")
	if !strings.Contains(err.Error(), "github.com refused the token") || strings.Contains(err.Error(), "auth login") {
		t.Errorf("text = %v", err)
	}
	if got := AsError(err).Args["account"]; got != "work" {
		t.Errorf("Args[account] = %v", got)
	}
}
