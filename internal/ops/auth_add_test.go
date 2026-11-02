package ops

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/github"
)

// cfgStore is a store that setConfig writes into config.yaml; account is
// empty for a store with no account: of its own.
type cfgStore struct{ name, account string }

// setConfig replaces config.yaml with head (accounts:, hosts:, ...) and the
// stores. The store "me" is the one of the test session; any other is only an
// entry, with a directory that does not exist.
func (ts *testSession) setConfig(head string, stores ...cfgStore) {
	ts.t.Helper()
	c, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	me, err := c.Select("me")
	if err != nil {
		ts.t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString("stores:\n")
	for _, s := range stores {
		path := me.Path
		if s.name != "me" {
			path = filepath.Join(ts.t.TempDir(), s.name)
		}
		fmt.Fprintf(&b, "  - {name: %s, path: %q", s.name, path)
		if s.account != "" {
			fmt.Fprintf(&b, ", account: %s", s.account)
		}
		b.WriteString("}\n")
	}
	if err := os.WriteFile(p.ConfigFile, []byte(b.String()), 0o600); err != nil {
		ts.t.Fatal(err)
	}
}

// storeAccounts is the account: of each store in config.yaml, by store name.
func (ts *testSession) storeAccounts() map[string]string {
	ts.t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	m := map[string]string{}
	for _, e := range c.Stores {
		m[e.Name] = e.Account
	}
	return m
}

// configBytes is config.yaml as it is on disk.
func (ts *testSession) configBytes() []byte {
	ts.t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.t.Fatal(err)
	}
	return b
}

// authFixture is authSession with config.yaml written whole: github.com on a
// fakegh, accounts, and the stores. The keychain is in memory.
func authFixture(t *testing.T, st fakegh.State, accounts string, stores ...cfgStore) (*testSession, *kr) {
	t.Helper()
	zero := 0
	if st.DeviceInterval == nil {
		st.DeviceInterval = &zero
	}
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	srv := httptest.NewServer(fakegh.New(st))
	t.Cleanup(srv.Close)
	ts := newTestSession(t)
	k := newKR()
	ts.Keyring = k
	ts.setConfig("hosts:\n  - {host: github.com, api: \""+srv.URL+"\", web: \""+srv.URL+"\", client_id: Iv23test}\n"+accounts, stores...)
	return ts, k
}

const personalAccount = "accounts:\n  - {name: personal, host: github.com, login: alice}\n"

func TestAuthAddFirstAccount(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.setConfig("", cfgStore{"me", ""})
	res, err := ts.AuthAdd(AuthAddInput{Name: "personal", Login: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "personal" || res.ID != "github.com/alice" || res.Token != "" || len(res.Pinned.Stores) != 0 {
		t.Errorf("result = %+v", res)
	}
	as := ts.accounts()
	if len(as) != 1 || as[0].Host != "github.com" || as[0].Login != "alice" || as[0].Token.Kind != "" {
		t.Errorf("accounts = %+v", as)
	}
	if strings.Contains(string(ts.configBytes()), "token") {
		t.Errorf("config.yaml = %s; want no token:", ts.configBytes())
	}
	// There was no account to keep for the store, so it is left as it is.
	if got := ts.storeAccounts(); got["me"] != "" {
		t.Errorf("store accounts = %v", got)
	}
}

// The token is not fetched: neither gh nor the keychain is asked, and the
// keychain is not written.
func TestAuthAddTokenSources(t *testing.T) {
	t.Parallel()
	for token, want := range map[string]string{"gh": "gh", "env:HH_TOKEN": "env:HH_TOKEN", "keychain": "keychain"} {
		t.Run(token, func(t *testing.T) {
			t.Parallel()
			ts := newTestSession(t)
			k := newKR()
			ts.Keyring = k
			ts.Run = func(name string, _ ...string) ([]byte, []byte, error) {
				t.Errorf("ran %s", name)
				return nil, nil, errors.New("must not run")
			}
			ts.setConfig("", cfgStore{"me", ""})
			res, err := ts.AuthAdd(AuthAddInput{Name: "work", Login: "bob", Host: " GitHub.com ", Token: " " + token + " "})
			if err != nil {
				t.Fatal(err)
			}
			if res.Token != want || res.ID != "GitHub.com/bob" {
				t.Errorf("result = %+v", res)
			}
			as := ts.accounts()
			if len(as) != 1 || as[0].Token.String() != want || as[0].Host != "GitHub.com" {
				t.Errorf("accounts = %+v", as)
			}
			if len(k.m) != 0 {
				t.Errorf("keychain = %v; want nothing written", k.m)
			}
		})
	}
}

// A second account would leave the stores with no account: to a reader of
// config.yaml, so they are given the one they used, in the same write.
func TestAuthAddPinsTheStoresOfTheOnlyAccount(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.setConfig(personalAccount, cfgStore{"me", ""}, cfgStore{"org-a", ""}, cfgStore{"org-b", "personal"})
	res, err := ts.AuthAdd(AuthAddInput{Name: "work", Login: "bob", Token: "gh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Pinned.Account != "personal" || !slices.Equal(res.Pinned.Stores, []string{"me", "org-a"}) {
		t.Errorf("Pinned = %+v; want personal on me and org-a (org-b had its own)", res.Pinned)
	}
	want := map[string]string{"me": "personal", "org-a": "personal", "org-b": "personal"}
	if got := ts.storeAccounts(); !maps.Equal(got, want) {
		t.Errorf("store accounts = %v, want %v", got, want)
	}
	if as := ts.accounts(); len(as) != 2 || as[1].Name != "work" {
		t.Errorf("accounts = %+v", as)
	}
	// A third account finds every store with an account already.
	res, err = ts.AuthAdd(AuthAddInput{Name: "extra", Login: "carol"})
	if err != nil || len(res.Pinned.Stores) != 0 {
		t.Errorf("third account: %+v, %v; want nothing pinned", res, err)
	}
}

func TestAuthAddRefusals(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.setConfig(personalAccount, cfgStore{"me", ""})
	before := ts.configBytes()
	for _, c := range []struct {
		name string
		in   AuthAddInput
		kind Kind
		code string
	}{
		{"same name", AuthAddInput{Name: "personal", Login: "bob"}, KindConflict, "account.exists"},
		{"same host and login, other case", AuthAddInput{Name: "other", Login: "ALICE", Host: "GitHub.com"}, KindConflict, "account.exists"},
		{"no name", AuthAddInput{Name: " ", Login: "bob"}, KindUsage, "auth.name"},
		{"no login", AuthAddInput{Name: "work"}, KindUsage, "auth.login"},
		{"device", AuthAddInput{Name: "work", Login: "bob", Token: "device"}, KindUsage, "auth.token_source"},
		{"empty env", AuthAddInput{Name: "work", Login: "bob", Token: "env:"}, KindUsage, "auth.token_source"},
		{"unknown", AuthAddInput{Name: "work", Login: "bob", Token: "nope"}, KindUsage, "auth.token_source"},
	} {
		_, err := ts.AuthAdd(c.in)
		t.Run(c.name, func(t *testing.T) { wantErr(t, err, c.kind, c.code) })
	}
	if after := ts.configBytes(); !bytes.Equal(before, after) {
		t.Errorf("config.yaml changed by refused adds:\n%s", after)
	}
}

// A login that adds an account gives the stores of the only account their
// account too; a login that only replaces a token does not.
func TestAuthLoginPinsTheStoresOfTheOnlyAccount(t *testing.T) {
	t.Parallel()
	ts, _ := authFixture(t, fakegh.State{Tokens: map[string]string{"t-bob": "bob"}}, personalAccount,
		cfgStore{"me", ""}, cfgStore{"org-a", ""})
	res, err := ts.AuthLogin(AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Added || res.Pinned.Account != "personal" || !slices.Equal(res.Pinned.Stores, []string{"me", "org-a"}) {
		t.Errorf("result = %+v", res)
	}
	want := map[string]string{"me": "personal", "org-a": "personal"}
	if got := ts.storeAccounts(); !maps.Equal(got, want) {
		t.Errorf("store accounts = %v, want %v", got, want)
	}
	res, err = ts.AuthLogin(AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
	if err != nil || res.Added || len(res.Pinned.Stores) != 0 {
		t.Errorf("second login = %+v, %v; want the token replaced and nothing pinned", res, err)
	}
}

// No client_id, no device flow: the login stops before it asks GitHub for a
// code, and writes nothing.
func TestAuthLoginDeviceNeedsClientID(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	k := newKR()
	ts.Keyring = k
	ts.setConfig("", cfgStore{"me", ""})
	before := ts.configBytes()
	_, err := ts.AuthLogin(AuthLoginInput{Host: "github.com", ShowCode: func(*github.DeviceCode) { t.Error("the code was shown") }})
	if err == nil || !strings.Contains(err.Error(), "the device flow needs hosts[].client_id for github.com in config.yaml") {
		t.Fatalf("err = %v", err)
	}
	if len(k.m) != 0 || !bytes.Equal(before, ts.configBytes()) {
		t.Errorf("keychain = %v, config.yaml changed = %v", k.m, !bytes.Equal(before, ts.configBytes()))
	}
}
