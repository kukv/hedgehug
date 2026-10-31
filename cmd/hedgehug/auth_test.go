package main

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/fakegh"
)

// withFake starts a fakegh with alice and bob, and points github.com of the
// config at it.
func (te *testEnv) withFake(st fakegh.State) *fakegh.Server { //nolint:unparam // later tests change the fake through the server
	te.t.Helper()
	zero := 0
	if st.DeviceInterval == nil {
		st.DeviceInterval = &zero
	}
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	s := fakegh.New(st)
	srv := httptest.NewServer(s)
	te.t.Cleanup(srv.Close)
	te.writeConfigYAML("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\", client_id: Iv23test}\n")
	return s
}

func (te *testEnv) configText() string {
	te.t.Helper()
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml")) //nolint:gosec // the config of the test
	return string(b)
}

func TestAuthLoginDevice(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "alice"})
	_, out, errs := te.run("auth", "login")
	if !strings.Contains(errs, "enter the code") {
		t.Errorf("stderr = %q", errs)
	}
	if !strings.Contains(out, "Logged in as github.com/alice") {
		t.Errorf("stdout = %q", out)
	}
	b := te.configText()
	if !strings.Contains(b, "login: alice") || !strings.Contains(b, "token: device") {
		t.Errorf("config.yaml = %s", b)
	}
	if out := te.ok("auth", "status"); !strings.Contains(out, "alice") || !strings.Contains(out, "device") || !strings.Contains(out, "ok") {
		t.Errorf("status = %q", out)
	}
	te.ok("auth", "logout", "alice")
	if code, _, _ := te.run("auth", "status"); code != 1 {
		t.Errorf("status after logout: exit %d", code)
	}
}

func TestAuthLoginWithToken(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{Tokens: map[string]string{"t-bob": "bob"}})
	te.stdin = "t-bob\n"
	out := te.ok("auth", "login", "work", "--with-token")
	if !strings.Contains(out, "github.com/bob") {
		t.Errorf("stdout = %q", out)
	}
	b := te.configText()
	if !strings.Contains(b, "name: work") || !strings.Contains(b, "token: keychain") {
		t.Errorf("config.yaml = %s", b)
	}
	if v, _ := keyring.Get("hedgehug", "github.com/bob"); v != "t-bob" {
		t.Errorf("keychain = %q", v)
	}
	// The same host and login again replaces the token, and adds no account.
	te.stdin = "t-bob\n"
	te.ok("auth", "login", "other", "--with-token")
	if b := te.configText(); strings.Count(b, "login: bob") != 1 {
		t.Errorf("config.yaml = %s", b)
	}
}

func TestAuthStatusMismatch(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{Tokens: map[string]string{"t-bob": "bob"}})
	te.env["HH_TOKEN"] = "t-bob"
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n  - {name: plain, host: github.com, login: x}\n")
	code, out, _ := te.run("auth", "status")
	if code != 1 || !strings.Contains(out, "bob") || !strings.Contains(out, "plain") {
		t.Errorf("exit %d, %q", code, out)
	}
	if out := te.ok("auth", "login", "me"); !strings.Contains(out, "Nothing to log in") {
		t.Errorf("env account login = %q", out)
	}
}

// An account in config.yaml with no token: (every account of phase 2 is one)
// gets its token from auth login, once GitHub says the token is of its login.
func TestAuthLoginFillsEmptyToken(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "alice"})
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice}\n")
	out := te.ok("auth", "login", "work")
	if !strings.Contains(out, "Logged in as github.com/alice (account work)") {
		t.Errorf("stdout = %q", out)
	}
	b := te.configText()
	if strings.Count(b, "login: alice") != 1 || !strings.Contains(b, "token: device") {
		t.Errorf("config.yaml = %s", b)
	}
	if v, err := keyring.Get("hedgehug", "github.com/alice"); err != nil || !strings.Contains(v, "access_token") {
		t.Errorf("keychain = %q, %v", v, err)
	}
	if out := te.ok("auth", "status"); !strings.Contains(out, "device") || !strings.Contains(out, "ok") {
		t.Errorf("status = %q", out)
	}
}

// A token of another login is refused, and nothing is written.
func TestAuthLoginEmptyTokenOfAnotherLogin(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "bob", Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"}})
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice}\n")
	noToken := func(what string) {
		t.Helper()
		if b := te.configText(); strings.Contains(b, "token:") {
			t.Errorf("%s: config.yaml has a token:\n%s", what, b)
		}
		for _, k := range []string{"github.com/alice", "github.com/bob"} {
			if _, err := keyring.Get("hedgehug", k); !errors.Is(err, keyring.ErrNotFound) {
				t.Errorf("%s: the keychain has %s: %v", what, k, err)
			}
		}
	}
	code, _, errs := te.run("auth", "login", "work") // the device flow approves bob
	if code != 1 || !strings.Contains(errs, "github.com/bob") || !strings.Contains(errs, "account work") {
		t.Errorf("device flow: exit %d, %q", code, errs)
	}
	noToken("device flow")
	te.stdin = "t-bob\n"
	code, _, errs = te.run("auth", "login", "work", "--with-token")
	if code != 1 || !strings.Contains(errs, "github.com/bob") {
		t.Errorf("--with-token: exit %d, %q", code, errs)
	}
	noToken("--with-token")
	// The right token is kept in the keychain.
	te.stdin = "t-alice\n"
	te.ok("auth", "login", "work", "--with-token")
	if b := te.configText(); !strings.Contains(b, "token: keychain") {
		t.Errorf("config.yaml = %s", b)
	}
	if v, _ := keyring.Get("hedgehug", "github.com/alice"); v != "t-alice" {
		t.Errorf("keychain = %q", v)
	}
}

// An account of the same host and login that uses gh takes the keychain or
// the device token instead, whatever name login was given.
func TestAuthLoginReplacesGh(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "alice", Tokens: map[string]string{"t-alice": "alice"}})
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice, token: gh}\n")
	te.stdin = "t-alice\n"
	te.ok("auth", "login", "other", "--with-token")
	b := te.configText()
	if strings.Count(b, "login: alice") != 1 || !strings.Contains(b, "token: keychain") || strings.Contains(b, "token: gh") {
		t.Errorf("config.yaml = %s", b)
	}
	// Now an account that keeps a token in the keychain: the device flow is
	// not for it, but a login of the same host and login by device replaces.
	te.ok("auth", "login", "other-name")
	if b := te.configText(); !strings.Contains(b, "token: device") || strings.Contains(b, "token: keychain") || strings.Count(b, "login: alice") != 1 {
		t.Errorf("config.yaml = %s", b)
	}
}

func TestAuthLoginWrongKind(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "alice"})
	te.writeConfigYAML("accounts:\n  - {name: k, host: github.com, login: alice, token: keychain}\n  - {name: d, host: github.com, login: bob, token: device}\n  - {name: g, host: github.com, login: x, token: gh}\n")
	code, _, errs := te.run("auth", "login", "k")
	if code != 2 || !strings.Contains(errs, "--with-token") {
		t.Errorf("keychain without --with-token: exit %d, %q", code, errs)
	}
	te.stdin = "t\n"
	code, _, errs = te.run("auth", "login", "d", "--with-token")
	if code != 2 || !strings.Contains(errs, "device flow") {
		t.Errorf("device with --with-token: exit %d, %q", code, errs)
	}
	if out := te.ok("auth", "login", "g"); !strings.Contains(out, "Nothing to log in for g") {
		t.Errorf("gh = %q", out)
	}
	code, _, errs = te.run("auth", "login", "k", "--host", "ghe.example.com", "--with-token")
	if code != 2 || !strings.Contains(errs, "github.com") {
		t.Errorf("--host of another host: exit %d, %q", code, errs)
	}
}

func TestAuthLoginEmptyStdin(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{})
	for _, in := range []string{"", "\n", "  \n"} {
		te.stdin = in
		code, _, errs := te.run("auth", "login", "--with-token")
		if code != 1 || !strings.Contains(errs, "no token on stdin") {
			t.Errorf("stdin %q: exit %d, %q", in, code, errs)
		}
	}
	// A last line with no newline is a token.
	te.stdin = "t-unknown"
	code, _, errs := te.run("auth", "login", "--with-token")
	if code != 1 || !strings.Contains(errs, "check the token") || !strings.Contains(errs, "401") {
		t.Errorf("unknown token: exit %d, %q", code, errs)
	}
	if b := te.configText(); strings.Contains(b, "accounts") {
		t.Errorf("config.yaml = %s", b)
	}
}

func TestAuthLoginOpensBrowser(t *testing.T) {
	for _, c := range []struct {
		name, stdin string
		terminal    bool
		opened      bool
	}{
		{"yes", "y\n", true, true},
		{"enter", "\n", true, true},
		{"no", "n\n", true, false},
		{"not a terminal", "y\n", false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			keyring.MockInit()
			te := newTestEnv(t)
			te.withFake(fakegh.State{AutoApprove: "alice"})
			var opened []string
			te.openURL = func(u string) error { opened = append(opened, u); return nil }
			te.terminal, te.stdin = c.terminal, c.stdin
			te.ok("auth", "login")
			if (len(opened) == 1) != c.opened || (c.opened && !strings.HasSuffix(opened[0], "/login/device")) {
				t.Errorf("opened = %v", opened)
			}
		})
	}
	t.Run("failure is a warning", func(t *testing.T) {
		keyring.MockInit()
		te := newTestEnv(t)
		te.withFake(fakegh.State{AutoApprove: "alice"})
		te.openURL = func(string) error { return errors.New("no browser") }
		te.terminal, te.stdin = true, "\n"
		_, _, errs := te.run("auth", "login")
		if !strings.Contains(errs, "no browser") {
			t.Errorf("stderr = %q", errs)
		}
		if !strings.Contains(te.configText(), "login: alice") {
			t.Error("the login did not go on")
		}
	})
}

func TestAuthJSON(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "alice"})
	te.writeConfigYAML("accounts:\n  - {name: nt, host: github.com, login: z}\n")
	var login struct {
		Account, ID string
		Added       bool
	}
	if err := json.Unmarshal([]byte(te.ok("auth", "login", "--json")), &login); err != nil || login.Account != "alice" || login.ID != "github.com/alice" || !login.Added {
		t.Errorf("login = %+v, %v", login, err)
	}
	if err := json.Unmarshal([]byte(te.ok("auth", "login", "alice", "--json")), &login); err != nil || login.Added {
		t.Errorf("second login = %+v, %v", login, err)
	}
	type statusJSON struct {
		Name, ID, Token string
		OK              bool
		Login, Error    string
		Expires         *string
	}
	var st []statusJSON
	if err := json.Unmarshal([]byte(te.ok("auth", "status", "--json")), &st); err != nil || len(st) != 2 {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if st[0].Name != "nt" || st[0].Token != "" || !st[0].OK || st[0].Expires != nil {
		t.Errorf("no token = %+v", st[0])
	}
	if st[1].Name != "alice" || st[1].Token != "device" || !st[1].OK || st[1].Login != "alice" || st[1].Expires == nil {
		t.Errorf("device = %+v", st[1])
	}
	var lo struct {
		Account string
		Removed bool
	}
	if err := json.Unmarshal([]byte(te.ok("auth", "logout", "alice", "--json")), &lo); err != nil || lo.Account != "alice" || !lo.Removed {
		t.Errorf("logout = %+v, %v", lo, err)
	}
	if err := json.Unmarshal([]byte(te.ok("auth", "logout", "nt", "--json")), &lo); err != nil || lo.Removed {
		t.Errorf("logout of no token = %+v, %v", lo, err)
	}
	code, out, _ := te.run("auth", "status", "--json")
	if code != 1 {
		t.Errorf("status after logout: exit %d", code)
	}
	st = nil
	if err := json.Unmarshal([]byte(out), &st); err != nil || len(st) != 2 || st[1].OK || st[1].Error == "" {
		t.Errorf("status after logout = %+v, %v", st, err)
	}
}

func TestAuthLogout(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.env["HH_TOKEN"] = "x"
	te.writeConfigYAML("accounts:\n  - {name: e, host: github.com, login: a, token: \"env:HH_TOKEN\"}\n  - {name: g, host: github.com, login: b, token: gh}\n  - {name: n, host: github.com, login: c}\n  - {name: k, host: github.com, login: d, token: keychain}\n")
	if err := keyring.Set("hedgehug", "github.com/d", "tok"); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"e", "g", "n"} {
		if out := te.ok("auth", "logout", n); !strings.Contains(out, "keeps nothing for "+n) {
			t.Errorf("logout %s = %q", n, out)
		}
	}
	if out := te.ok("auth", "logout", "k"); !strings.Contains(out, "Removed the token of github.com/d") {
		t.Errorf("logout k = %q", out)
	}
	if _, err := keyring.Get("hedgehug", "github.com/d"); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("keychain: %v", err)
	}
	// The account stays in config.yaml, and a second logout is no error.
	if b := te.configText(); !strings.Contains(b, "name: k") {
		t.Errorf("config.yaml = %s", b)
	}
	te.ok("auth", "logout", "k")
	if code, _, errs := te.run("auth", "logout", "nobody"); code != 1 || !strings.Contains(errs, `"nobody"`) {
		t.Errorf("unknown: exit %d, %q", code, errs)
	}
	if code, _, _ := te.run("auth", "logout"); code != 2 {
		t.Errorf("no name: exit %d", code)
	}
}

func TestAuthStatusEmptyAndBare(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	if out := te.ok("auth", "status"); !strings.Contains(out, "auth login") {
		t.Errorf("status = %q", out)
	}
	if out := te.ok("auth", "status", "--json"); strings.TrimSpace(out) != "[]" {
		t.Errorf("status --json = %q", out)
	}
	if code, _, errs := te.run("auth"); code != 2 || !strings.Contains(errs, "subcommand") {
		t.Errorf("auth: exit %d, %q", code, errs)
	}
}

// A refused token, and a keychain with none, are failed statuses that say
// what to run.
func TestAuthStatusRefused(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{})
	te.env["HH_TOKEN"] = "unknown"
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n  - {name: gone, host: github.com, login: bob, token: keychain}\n")
	code, out, _ := te.run("auth", "status")
	if code != 1 || !strings.Contains(out, "auth login me") || !strings.Contains(out, "auth login gone --with-token") {
		t.Errorf("exit %d, %q", code, out)
	}
}
