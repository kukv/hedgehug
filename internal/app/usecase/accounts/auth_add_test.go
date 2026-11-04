package accounts_test

import (
	"bytes"
	"errors"
	"maps"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// authFixture is AuthSession with config.yaml written whole: github.com on a
// fakegh, accounts, and the stores. The keychain is in memory.
func authFixture(t *testing.T, st fakegh.State, accounts string, stores ...usecasetest.CfgStore) (*usecasetest.Session, *usecasetest.Keyring) {
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
	ts := usecasetest.New(t)
	k := usecasetest.NewKeyring()
	ts.Keyring = k
	ts.SetConfig("hosts:\n  - {host: github.com, api: \""+srv.URL+"\", web: \""+srv.URL+"\", client_id: Iv23test}\n"+accounts, stores...)
	return ts, k
}

const personalAccount = "accounts:\n  - {name: personal, host: github.com, login: alice}\n"

func TestAuthAddFirstAccount(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.SetConfig("", usecasetest.CfgStore{Name: "me", Account: ""})
	res, err := accounts.AuthAdd(ts.Session, accounts.AuthAddInput{Name: "personal", Login: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "personal" || res.ID != "github.com/alice" || res.Token != "" || len(res.Pinned.Stores) != 0 {
		t.Errorf("result = %+v", res)
	}
	as := ts.Accounts()
	if len(as) != 1 || as[0].Host != "github.com" || as[0].Login != "alice" || as[0].Token.Kind != "" {
		t.Errorf("accounts = %+v", as)
	}
	if strings.Contains(string(ts.ConfigBytes()), "token") {
		t.Errorf("config.yaml = %s; want no token:", ts.ConfigBytes())
	}
	// There was no account to keep for the store, so it is left as it is.
	if got := ts.StoreAccounts(); got["me"] != "" {
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
			ts := usecasetest.New(t)
			k := usecasetest.NewKeyring()
			ts.Keyring = k
			ts.Run = func(name string, _ ...string) ([]byte, []byte, error) {
				t.Errorf("ran %s", name)
				return nil, nil, errors.New("must not run")
			}
			ts.SetConfig("", usecasetest.CfgStore{Name: "me", Account: ""})
			res, err := accounts.AuthAdd(ts.Session, accounts.AuthAddInput{Name: "work", Login: "bob", Host: " GitHub.com ", Token: " " + token + " "})
			if err != nil {
				t.Fatal(err)
			}
			if res.Token != want || res.ID != "GitHub.com/bob" {
				t.Errorf("result = %+v", res)
			}
			as := ts.Accounts()
			if len(as) != 1 || as[0].Token.String() != want || as[0].Host != "GitHub.com" {
				t.Errorf("accounts = %+v", as)
			}
			if len(k.M) != 0 {
				t.Errorf("keychain = %v; want nothing written", k.M)
			}
		})
	}
}

// A second account would leave the stores with no account: to a reader of
// config.yaml, so they are given the one they used, in the same write.
func TestAuthAddPinsTheStoresOfTheOnlyAccount(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.SetConfig(personalAccount, usecasetest.CfgStore{Name: "me", Account: ""}, usecasetest.CfgStore{Name: "org-a", Account: ""}, usecasetest.CfgStore{Name: "org-b", Account: "personal"})
	res, err := accounts.AuthAdd(ts.Session, accounts.AuthAddInput{Name: "work", Login: "bob", Token: "gh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Pinned.Account != "personal" || !slices.Equal(res.Pinned.Stores, []string{"me", "org-a"}) {
		t.Errorf("Pinned = %+v; want personal on me and org-a (org-b had its own)", res.Pinned)
	}
	want := map[string]string{"me": "personal", "org-a": "personal", "org-b": "personal"}
	if got := ts.StoreAccounts(); !maps.Equal(got, want) {
		t.Errorf("store accounts = %v, want %v", got, want)
	}
	if as := ts.Accounts(); len(as) != 2 || as[1].Name != "work" {
		t.Errorf("accounts = %+v", as)
	}
	// A third account finds every store with an account already.
	res, err = accounts.AuthAdd(ts.Session, accounts.AuthAddInput{Name: "extra", Login: "carol"})
	if err != nil || len(res.Pinned.Stores) != 0 {
		t.Errorf("third account: %+v, %v; want nothing pinned", res, err)
	}
}

func TestAuthAddRefusals(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.SetConfig(personalAccount, usecasetest.CfgStore{Name: "me", Account: ""})
	before := ts.ConfigBytes()
	for _, c := range []struct {
		name string
		in   accounts.AuthAddInput
		kind usecase.Kind
		code string
	}{
		{"same name", accounts.AuthAddInput{Name: "personal", Login: "bob"}, usecase.KindConflict, "account.exists"},
		{"same host and login, other case", accounts.AuthAddInput{Name: "other", Login: "ALICE", Host: "GitHub.com"}, usecase.KindConflict, "account.exists"},
		{"no name", accounts.AuthAddInput{Name: " ", Login: "bob"}, usecase.KindUsage, "auth.name"},
		{"no login", accounts.AuthAddInput{Name: "work"}, usecase.KindUsage, "auth.login"},
		{"device", accounts.AuthAddInput{Name: "work", Login: "bob", Token: "device"}, usecase.KindUsage, "auth.token_source"},
		{"empty env", accounts.AuthAddInput{Name: "work", Login: "bob", Token: "env:"}, usecase.KindUsage, "auth.token_source"},
		{"unknown", accounts.AuthAddInput{Name: "work", Login: "bob", Token: "nope"}, usecase.KindUsage, "auth.token_source"},
	} {
		_, err := accounts.AuthAdd(ts.Session, c.in)
		t.Run(c.name, func(t *testing.T) { usecasetest.WantErr(t, err, c.kind, c.code) })
	}
	if after := ts.ConfigBytes(); !bytes.Equal(before, after) {
		t.Errorf("config.yaml changed by refused adds:\n%s", after)
	}
}

// A login that adds an account gives the stores of the only account their
// account too; a login that only replaces a token does not.
func TestAuthLoginPinsTheStoresOfTheOnlyAccount(t *testing.T) {
	t.Parallel()
	ts, _ := authFixture(t, fakegh.State{Tokens: map[string]string{"t-bob": "bob"}}, personalAccount,
		usecasetest.CfgStore{Name: "me", Account: ""}, usecasetest.CfgStore{Name: "org-a", Account: ""})
	res, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Added || res.Pinned.Account != "personal" || !slices.Equal(res.Pinned.Stores, []string{"me", "org-a"}) {
		t.Errorf("result = %+v", res)
	}
	want := map[string]string{"me": "personal", "org-a": "personal"}
	if got := ts.StoreAccounts(); !maps.Equal(got, want) {
		t.Errorf("store accounts = %v, want %v", got, want)
	}
	res, err = accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
	if err != nil || res.Added || len(res.Pinned.Stores) != 0 {
		t.Errorf("second login = %+v, %v; want the token replaced and nothing pinned", res, err)
	}
}

// No client_id, no device flow: the login stops before it asks GitHub for a
// code, and writes nothing.
func TestAuthLoginDeviceNeedsClientID(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	k := usecasetest.NewKeyring()
	ts.Keyring = k
	ts.SetConfig("", usecasetest.CfgStore{Name: "me", Account: ""})
	before := ts.ConfigBytes()
	_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com", ShowCode: func(*github.DeviceCode) { t.Error("the code was shown") }})
	if err == nil || !strings.Contains(err.Error(), "the device flow needs hosts[].client_id for github.com in config.yaml") {
		t.Fatalf("err = %v", err)
	}
	if len(k.M) != 0 || !bytes.Equal(before, ts.ConfigBytes()) {
		t.Errorf("keychain = %v, config.yaml changed = %v", k.M, !bytes.Equal(before, ts.ConfigBytes()))
	}
}
