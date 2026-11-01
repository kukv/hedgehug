package ops

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/config"
)

func TestConn(t *testing.T) {
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "tok-1"
	ts.appendConfig("accounts:\n" +
		"  - {name: a, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"  - {name: b, host: github.com, login: bob}\n" +
		"url_rewrites:\n  - {from: \"https://github.com/\", to: \"/srv/remotes/\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	conn := ts.Conn(c, config.StoreEntry{Account: "a"})
	if got := conn.Rewrite("https://github.com/org/x.git"); got != "/srv/remotes/org/x.git" {
		t.Errorf("Rewrite = %q", got)
	}
	for _, u := range []string{"git@github.com:org/x.git", "ssh://git@github.com/org/x.git", "/srv/remotes/org/x.git", "file:///srv/x.git"} {
		if au, err := conn.Auth(u); au != nil || err != nil {
			t.Errorf("Auth(%q) = %v, %v; want none", u, au, err)
		}
	}
	au, err := conn.Auth("https://github.com/org/x.git")
	if b, ok := au.(*githttp.BasicAuth); err != nil || !ok || b.Username != "x-access-token" || b.Password != "tok-1" {
		t.Errorf("Auth(https) = %#v, %v", au, err)
	}
	// The token is got once a run: it stays when the variable goes.
	ts.env["HH_TOKEN"] = ""
	au, err = conn.Auth("http://github.com/org/x.git")
	if b, ok := au.(*githttp.BasicAuth); err != nil || !ok || b.Password != "tok-1" {
		t.Errorf("second Auth = %#v, %v", au, err)
	}
	// Only the account's own host gets the token, whatever the case or port.
	for u, want := range map[string]bool{
		"https://github.com/org/x.git":      true,
		"https://GitHub.com:8443/org/x.git": true,
		"http://github.com/org/x.git":       true,
		"https://mirror.example.com/x.git":  false,
		"https://github.com.evil.test/x":    false,
		"https://api.github.com/x":          false,
	} {
		au, err := conn.Auth(u)
		if err != nil || (au != nil) != want {
			t.Errorf("Auth(%q) = %v, %v; want token %v", u, au, err, want)
		}
	}
	// An account with no token, and a name that is no account.
	for _, name := range []string{"b", "nobody"} {
		if au, err := ts.Conn(c, config.StoreEntry{Account: name}).Auth("https://github.com/org/x.git"); au != nil || err != nil {
			t.Errorf("account %q: Auth = %v, %v; want none", name, au, err)
		}
	}
}

// A token that cannot be got is an error, and is asked for again.
func TestConnMissingTokenIsAskedAgain(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig("accounts:\n  - {name: a, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := ts.Conn(c, config.StoreEntry{Account: "a"}).Auth("https://github.com/org/x.git"); err == nil || !strings.Contains(err.Error(), "HH_TOKEN is not set") {
			t.Errorf("a missing token = %v", err)
		}
	}
}

type fakeKeyring struct{}

func (*fakeKeyring) Get(string, string) (string, error) { return "", errors.New("none") }
func (*fakeKeyring) Set(string, string, string) error   { return nil }
func (*fakeKeyring) Delete(string, string) error        { return nil }

// The session hands what it was given to the token code.
func TestAuthOptions(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig("hosts:\n  - {host: ghe.example.com, api: \"https://ghe.example.com/api\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	k := &fakeKeyring{}
	ts.Keyring = k
	ts.Run = func(string, ...string) ([]byte, []byte, error) { return nil, nil, nil }
	opt := ts.AuthOptions(c, config.Account{Host: "ghe.example.com"})
	if opt.Keyring != k || opt.Run == nil || opt.Host.API != "https://ghe.example.com/api" {
		t.Errorf("options = %+v", opt)
	}
	if !strings.HasSuffix(opt.LockFile, "auth.lock") {
		t.Errorf("LockFile = %q", opt.LockFile)
	}
}

// The token of an account is got once for the session and the sessions
// ForStore makes, even when they ask at once.
func TestTokenIsGotOnceAcrossForStore(t *testing.T) {
	ts := newTestSession(t)
	var calls atomic.Int32
	ts.Getenv = func(k string) string {
		if k == "HH_TOKEN" {
			calls.Add(1)
			return " tok-1 "
		}
		return ts.env[k]
	}
	ts.appendConfig("accounts:\n  - {name: a, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	acct, _ := c.Account("a")
	child := ts.ForStore("me")
	done := make(chan string, 8)
	for i := range 8 {
		ss := ts.Session
		if i%2 == 1 {
			ss = child
		}
		go func() {
			tok, err := ss.Token(c, acct)
			if err != nil {
				tok = err.Error()
			}
			done <- tok
		}()
	}
	for range 8 {
		if tok := <-done; tok != "tok-1" {
			t.Errorf("Token = %q", tok)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("the variable was read %d times; want 1", n)
	}
}

// A slow fetch of one account does not hold up the others.
func TestTokenFetchOfOneAccountDoesNotBlockAnother(t *testing.T) {
	ts := newTestSession(t)
	started, release := make(chan struct{}), make(chan struct{})
	ts.Run = func(string, ...string) ([]byte, []byte, error) {
		close(started)
		<-release
		return []byte("gh-tok"), nil, nil
	}
	ts.env["HH_TOKEN"] = "env-tok"
	ts.appendConfig("accounts:\n" +
		"  - {name: slow, host: github.com, login: alice, token: gh}\n" +
		"  - {name: fast, host: github.com, login: bob, token: \"env:HH_TOKEN\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	slow, _ := c.Account("slow")
	fast, _ := c.Account("fast")
	done := make(chan error, 1)
	go func() { _, err := ts.Token(c, slow); done <- err }()
	<-started
	got := make(chan string, 1)
	go func() { tok, _ := ts.Token(c, fast); got <- tok }()
	select {
	case tok := <-got:
		if tok != "env-tok" {
			t.Errorf("fast token = %q", tok)
		}
	case <-time.After(5 * time.Second):
		t.Error("the fast account waited for the slow one")
	}
	close(release)
	if err := <-done; err != nil {
		t.Error(err)
	}
}

// The same name with another host or login is another account: a session
// that outlives a config change does not hand out the old account's token.
func TestTokenCacheKeyIncludesTheAccountID(t *testing.T) {
	ts := newTestSession(t)
	ts.env["A"], ts.env["B"] = "tok-a", "tok-b"
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	a := config.Account{Name: "x", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "env", Env: "A"}}
	b := config.Account{Name: "x", Host: "github.com", Login: "bob", Token: config.TokenSource{Kind: "env", Env: "B"}}
	if tok, err := ts.Token(c, a); err != nil || tok != "tok-a" {
		t.Fatalf("a = %q, %v", tok, err)
	}
	if tok, err := ts.Token(c, b); err != nil || tok != "tok-b" {
		t.Errorf("same name, other login = %q, %v; want tok-b", tok, err)
	}
	// The ID is compared without case, as everywhere.
	a2 := a
	a2.Login = "ALICE"
	ts.env["A"] = "changed"
	if tok, _ := ts.Token(c, a2); tok != "tok-a" {
		t.Errorf("alice and ALICE are one account; token = %q", tok)
	}
}

func TestForStoreCopiesTheOutsideWorld(t *testing.T) {
	ts := newTestSession(t)
	k := &fakeKeyring{}
	ts.Keyring = k
	ts.Run = func(string, ...string) ([]byte, []byte, error) { return nil, nil, nil }
	child := ts.ForStore("x")
	if child.Keyring != k || child.Run == nil || child.StoreName != "x" {
		t.Errorf("ForStore = %+v", child)
	}
}

func TestGitHub(t *testing.T) {
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "tok-1"
	ts.appendConfig("accounts:\n" +
		"  - {name: a, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"  - {name: b, host: github.com, login: bob}\n" +
		"  - {name: c, host: github.com, login: carol, token: \"env:NOPE\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := c.Account("a")
	if gh, err := ts.GitHub(c, a, "github.com"); err != nil || gh == nil {
		t.Errorf("GitHub = %v, %v", gh, err)
	}
	b, _ := c.Account("b")
	_, err = ts.GitHub(c, b, "github.com")
	if err == nil || AsError(err).Code != "auth.token" || !strings.Contains(err.Error(), "hedgehug auth login b") {
		t.Errorf("an account with no token: %v", err)
	}
	if k := KindOf(err); k != KindInvalid {
		t.Errorf("kind = %v; want KindInvalid", k)
	}
	cc, _ := c.Account("c")
	if _, err := ts.GitHub(c, cc, "github.com"); err == nil || !strings.Contains(err.Error(), "NOPE is not set") {
		t.Errorf("a missing token = %v", err)
	}
}

func TestPickAccount(t *testing.T) {
	ts := newTestSession(t)
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := ts.PickAccount(c, ""); ok || err != nil {
		t.Errorf("no accounts: ok=%v err=%v", ok, err)
	}
	ts.appendConfig("accounts:\n  - {name: a, host: github.com, login: alice}\n")
	c, _, _ = ts.Config()
	if acct, ok, err := ts.PickAccount(c, ""); !ok || err != nil || acct.Name != "a" {
		t.Errorf("the only account: %v %v %v", acct, ok, err)
	}
	ts.appendConfig("  - {name: b, host: github.com, login: bob}\n")
	c, _, _ = ts.Config()
	if _, ok, err := ts.PickAccount(c, ""); ok || err != nil {
		t.Errorf("two accounts, no name: ok=%v err=%v", ok, err)
	}
	if acct, ok, err := ts.PickAccount(c, "b"); !ok || err != nil || acct.Name != "b" {
		t.Errorf("b: %v %v %v", acct, ok, err)
	}
	_, ok, err := ts.PickAccount(c, "nobody")
	ae := AsError(err)
	if ok || ae == nil || ae.Kind != KindNotFound || ae.Code != "account.not_found" || ae.Args["name"] != "nobody" {
		t.Fatalf("nobody: ok=%v err=%v", ok, err)
	}
	if got, _ := ae.Args["candidates"].([]string); strings.Join(got, ",") != "a,b" {
		t.Errorf("candidates = %v", ae.Args["candidates"])
	}
	if !strings.Contains(err.Error(), `"nobody"`) || !strings.Contains(err.Error(), "a, b") {
		t.Errorf("message = %q", err)
	}
}
