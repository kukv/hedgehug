package accounts_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/remote/auth"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/osfs"
)

func TestAuthLoginNewAccountByDevice(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{AutoApprove: "alice"}, "")
	var shown accounts.DeviceCode
	res, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com", OnCode: func(dc accounts.DeviceCode) { shown = dc }})
	if err != nil {
		t.Fatal(err)
	}
	if res.Account != "alice" || res.ID != "github.com/alice" || !res.Added {
		t.Errorf("result = %+v", res)
	}
	if shown.UserCode == "" {
		t.Errorf("OnCode got %+v", shown)
	}
	if as := ts.Accounts(); len(as) != 1 || as[0].Token.Kind != "device" {
		t.Errorf("accounts = %+v", as)
	}
	if v, ok := k.Has("github.com/alice"); !ok || !strings.Contains(v, "access_token") {
		t.Errorf("keychain = %q, %v", v, ok)
	}
}

// The login is compared without case: an account written Alice takes the
// token of alice, and keeps its own spelling, in config.yaml and as the key.
func TestAuthLoginLoginCaseIsIgnored(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{AutoApprove: "alice", Tokens: map[string]string{"t-alice": "alice"}},
		"accounts:\n  - {name: work, host: github.com, login: Alice}\n")
	if _, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "work", Host: "github.com"}); err != nil {
		t.Fatalf("by name: %v", err)
	}
	if _, ok := k.Has("github.com/Alice"); !ok {
		t.Errorf("keychain has %v; want the key in the spelling of config.yaml", k.M)
	}
	// Another name, the same host and login in another case: no new account.
	res, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "other", Host: "github.com", WithToken: true, Token: " t-alice \n"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added || res.Account != "work" {
		t.Errorf("result = %+v; want the token of work replaced", res)
	}
	as := ts.Accounts()
	if len(as) != 1 || as[0].Login != "Alice" || as[0].Token.Kind != "keychain" {
		t.Errorf("accounts = %+v", as)
	}
	if v, _ := k.Has("github.com/Alice"); v != "t-alice" {
		t.Errorf("keychain = %q; want the trimmed token", v)
	}
}

// Two accounts of one host and login in config.yaml stop the load, so a
// login cannot go on from them.
func TestAuthLoginRefusesDuplicateAccounts(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{Tokens: map[string]string{"t-alice": "alice"}},
		"accounts:\n  - {name: a, host: github.com, login: alice}\n  - {name: b, host: GitHub.com, login: ALICE}\n")
	_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com", WithToken: true, Token: "t-alice"})
	if err == nil || !strings.Contains(err.Error(), "listed twice") {
		t.Fatalf("err = %v", err)
	}
	if len(k.M) != 0 {
		t.Errorf("keychain = %v", k.M)
	}
}

func TestAuthLoginHostMustBeTheAccounts(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{Tokens: map[string]string{"t-alice": "alice"}},
		"accounts:\n  - {name: work, host: github.com, login: alice, token: keychain}\n")
	_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "work", Host: "ghe.example.com", HostGiven: true, WithToken: true, Token: "t-alice"})
	usecasetest.WantErr(t, err, usecase.KindUsage, "")
	if len(k.M) != 0 {
		t.Errorf("keychain = %v", k.M)
	}
	// The same host in another case is the same host.
	if _, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "work", Host: "GitHub.com", HostGiven: true, WithToken: true, Token: "t-alice"}); err != nil {
		t.Errorf("same host, other case: %v", err)
	}
}

func TestAuthLoginKinds(t *testing.T) {
	t.Parallel()
	ts, _ := usecasetest.AuthSession(t, fakegh.State{AutoApprove: "alice"},
		"accounts:\n  - {name: k, host: github.com, login: alice, token: keychain}\n  - {name: d, host: github.com, login: bob, token: device}\n  - {name: g, host: github.com, login: x, token: gh}\n  - {name: e, host: github.com, login: y, token: \"env:V\"}\n")
	_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "k", Host: "github.com"})
	usecasetest.WantErr(t, err, usecase.KindUsage, "")
	if !strings.Contains(err.Error(), "--with-token") {
		t.Errorf("keychain without a token: %v", err)
	}
	_, err = accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "d", Host: "github.com", WithToken: true, Token: "t"})
	usecasetest.WantErr(t, err, usecase.KindUsage, "")
	if !strings.Contains(err.Error(), "device flow") {
		t.Errorf("device with a token: %v", err)
	}
	for name, uses := range map[string]string{"g": "gh", "e": "env:V"} {
		res, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: name, Host: "github.com"})
		if err != nil || res.Uses != uses || res.Account != name {
			t.Errorf("%s: %+v, %v", name, res, err)
		}
	}
}

func TestAuthLoginEmptyToken(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{}, "")
	for _, tok := range []string{"", "  \n"} {
		_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com", WithToken: true, Token: tok})
		if err == nil || !strings.Contains(err.Error(), "no token on stdin") {
			t.Errorf("token %q: %v", tok, err)
		}
	}
	if len(k.M) != 0 || len(ts.Accounts()) != 0 {
		t.Errorf("keychain %v, accounts %v", k.M, ts.Accounts())
	}
}

func TestAuthLoginOtherLoginWritesNothing(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{AutoApprove: "bob", Tokens: map[string]string{"t-bob": "bob"}},
		"accounts:\n  - {name: work, host: github.com, login: alice}\n")
	for _, in := range []accounts.AuthLoginInput{
		{Name: "work", Host: "github.com"},
		{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"},
	} {
		_, err := accounts.AuthLogin(ts.Session, in)
		if err == nil || !strings.Contains(err.Error(), "github.com/bob") || !strings.Contains(err.Error(), "account work") {
			t.Errorf("%+v: %v", in, err)
		}
	}
	if len(k.M) != 0 || ts.Accounts()[0].Token.Kind != "" {
		t.Errorf("keychain %v, accounts %+v", k.M, ts.Accounts())
	}
}

// breakConfig makes the config updates fail: the directory of config.yaml
// cannot be written to, so the file cannot be replaced. The lock file of the
// directory is made first, so that the login still gets to the keychain.
func breakConfig(t *testing.T, ts *usecasetest.Session) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("a directory that cannot be written to needs a permission that is enforced")
	}
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := osfs.LockFile(ts.LockFile(), 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(p.ConfigFile)
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, fi.Mode().Perm()) }) // so that the directory of the test can be removed
}

// A login that cannot write config.yaml puts the keychain back: to the old
// value, or to nothing when there was none.
func TestAuthLoginRollsBackTheKeychain(t *testing.T) {
	t.Parallel()
	t.Run("old value", func(t *testing.T) {
		ts, k := usecasetest.AuthSession(t, fakegh.State{Tokens: map[string]string{"t-alice": "alice"}},
			"accounts:\n  - {name: work, host: github.com, login: alice, token: keychain}\n")
		k.M["github.com/alice"] = "old-secret"
		breakConfig(t, ts)
		if _, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-alice"}); err == nil {
			t.Fatal("no error")
		}
		if v, _ := k.Has("github.com/alice"); v != "old-secret" {
			t.Errorf("keychain = %q; want the old value", v)
		}
	})
	t.Run("no old value", func(t *testing.T) {
		ts, k := usecasetest.AuthSession(t, fakegh.State{AutoApprove: "alice"}, "")
		breakConfig(t, ts)
		if _, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com"}); err == nil {
			t.Fatal("no error")
		}
		if v, ok := k.Has("github.com/alice"); ok {
			t.Errorf("keychain = %q; want nothing", v)
		}
	})
	t.Run("another writer added the account", func(t *testing.T) {
		ts, k := usecasetest.AuthSession(t, fakegh.State{Tokens: map[string]string{"t-alice": "alice"}}, "")
		usecasetest.HookConfigWrite(t, ts, func(c *config.Config) error {
			return c.AddAccount(config.Account{Name: "theirs", Host: "github.com", Login: "Alice"})
		})
		_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Name: "mine", Host: "github.com", WithToken: true, Token: "t-alice"})
		usecasetest.WantErr(t, err, usecase.KindConflict, "")
		if v, ok := k.Has("github.com/alice"); ok {
			t.Errorf("keychain = %q; want nothing", v)
		}
		if as := ts.Accounts(); len(as) != 1 || as[0].Name != "theirs" {
			t.Errorf("accounts = %+v", as)
		}
	})
}

func TestAuthLoginRollbackFailureIsSaid(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{AutoApprove: "alice"}, "")
	breakConfig(t, ts)
	k.FailDelete = errors.New("keychain locked")
	_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com"})
	if err == nil || !strings.Contains(err.Error(), "keychain locked") || !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("err = %v; want both failures", err)
	}
}

// The keychain write of a login holds auth.lock; the polling of the device
// flow does not.
func TestAuthLoginLocksTheSave(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{AutoApprove: "alice", Tokens: map[string]string{"t-alice": "alice"}}, "")
	lock := ts.LockFile()
	var heldDuringShow, heldDuringSet []bool
	k.OnSet = func() { heldDuringSet = append(heldDuringSet, usecasetest.HeldElsewhere(lock)) }
	_, err := accounts.AuthLogin(ts.Session, accounts.AuthLoginInput{Host: "github.com", OnCode: func(accounts.DeviceCode) {
		heldDuringShow = append(heldDuringShow, usecasetest.HeldElsewhere(lock))
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(heldDuringShow) != 1 || heldDuringShow[0] {
		t.Errorf("held during the device flow = %v; want not held", heldDuringShow)
	}
	if len(heldDuringSet) != 1 || !heldDuringSet[0] {
		t.Errorf("held during the save = %v; want held", heldDuringSet)
	}
}

func TestAuthLogout(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{},
		"accounts:\n  - {name: d, host: github.com, login: dev, token: device}\n  - {name: k, host: github.com, login: key, token: keychain}\n  - {name: g, host: github.com, login: gh1, token: gh}\n  - {name: n, host: github.com, login: none1}\n")
	k.M["github.com/dev"], k.M["github.com/key"] = "{}", "tok"
	lock := ts.LockFile()
	var held []bool
	k.OnDelete = func() { held = append(held, usecasetest.HeldElsewhere(lock)) }
	for _, name := range []string{"d", "k"} {
		res, err := accounts.AuthLogout(ts.Session, name)
		removed := res.Removed
		if err != nil || !removed {
			t.Errorf("%s: %v, %v", name, removed, err)
		}
	}
	if len(k.M) != 0 || len(held) != 2 || !held[0] || !held[1] {
		t.Errorf("keychain %v, lock held during delete %v", k.M, held)
	}
	// Again: nothing to delete is no error.
	if res, err := accounts.AuthLogout(ts.Session, "d"); err != nil || !res.Removed {
		t.Errorf("again: %+v, %v", res, err)
	}
	// The result names the account and where its token comes from, so that
	// the CLI prints without reading config.yaml again.
	for name, want := range map[string]accounts.AuthLogoutResult{
		"g": {Account: "g", ID: "github.com/gh1", Token: "gh"},
		"n": {Account: "n", ID: "github.com/none1"},
		"d": {Account: "d", ID: "github.com/dev", Token: "device", Removed: true},
	} {
		if res, err := accounts.AuthLogout(ts.Session, name); err != nil || res != want {
			t.Errorf("%s: %+v, %v; want %+v", name, res, err, want)
		}
	}
	_, err := accounts.AuthLogout(ts.Session, "nobody")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "account.not_found")
	if as := ts.Accounts(); len(as) != 4 {
		t.Errorf("accounts = %+v; they stay", as)
	}
}

// An empty name is refused before config.yaml is read: a broken file does not
// change the answer.
func TestAuthLogoutNameFirst(t *testing.T) {
	t.Parallel()
	ts, _ := usecasetest.AuthSession(t, fakegh.State{}, "")
	_, p, _ := ts.Config()
	if err := os.WriteFile(p.ConfigFile, []byte("accounts: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := accounts.AuthLogout(ts.Session, "")
	usecasetest.WantErr(t, err, usecase.KindUsage, "auth.name")
}

func TestAuthStatus(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"}},
		"accounts:\n"+
			"  - {name: forever, host: github.com, login: alice, token: device}\n"+
			"  - {name: limited, host: github.com, login: bob, token: device}\n"+
			"  - {name: plain, host: github.com, login: zed}\n"+
			"  - {name: gone, host: github.com, login: yan, token: keychain}\n"+
			"  - {name: wrong, host: github.com, login: xi, token: keychain}\n")
	end := time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC)
	forever, _ := json.Marshal(auth.DeviceToken{AccessToken: "t-alice"})
	limited, _ := json.Marshal(auth.DeviceToken{AccessToken: "t-bob", ExpiresAt: end})
	k.M["github.com/alice"], k.M["github.com/bob"] = string(forever), string(limited)
	k.M["github.com/xi"] = "t-alice" // a token of alice under an account of xi
	got, err := accounts.AuthStatuses(ts.Session)
	if !errors.Is(err, usecase.ErrFindings) {
		t.Fatalf("err = %v; want ErrFindings", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %+v", got)
	}
	raw := func(i int) map[string]any {
		b, _ := json.Marshal(got[i])
		m := map[string]any{}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	// A device token with no expires_at: no expires, state ok.
	m := raw(0)
	if _, ok := m["expires"]; ok || m["state"] != "ok" || m["login"] != "alice" || m["token"] != "device" {
		t.Errorf("forever = %v", m)
	}
	if _, ok := m["ok"]; ok {
		t.Errorf("forever has the old ok key: %v", m)
	}
	m = raw(1)
	if m["state"] != "ok" || got[1].Expires == nil || !got[1].Expires.Equal(end) {
		t.Errorf("limited = %v", m)
	}
	// No token: only name, id and state "none".
	m = raw(2)
	if len(m) != 3 || m["state"] != "none" || m["name"] != "plain" || m["id"] != "github.com/zed" {
		t.Errorf("plain = %v", m)
	}
	if m = raw(3); m["state"] != "error" || !strings.Contains(got[3].Error, "auth login gone --with-token") || got[3].Login != "" {
		t.Errorf("gone = %v", m)
	}
	if m = raw(4); m["state"] != "error" || !strings.Contains(got[4].Error, "github.com/alice") {
		t.Errorf("wrong = %v", m)
	}
}

func TestAuthStatusAllGood(t *testing.T) {
	t.Parallel()
	ts, k := usecasetest.AuthSession(t, fakegh.State{Tokens: map[string]string{"t-alice": "alice"}},
		"accounts:\n  - {name: a, host: github.com, login: Alice, token: keychain}\n  - {name: p, host: github.com, login: zed}\n")
	k.M["github.com/Alice"] = "t-alice" // a login in another case than GitHub's
	got, err := accounts.AuthStatuses(ts.Session)
	if err != nil || len(got) != 2 || got[0].State != "ok" || got[1].State != "none" {
		t.Errorf("got %+v, %v", got, err)
	}
	empty, _ := usecasetest.AuthSession(t, fakegh.State{}, "")
	if got, err := accounts.AuthStatuses(empty.Session); err != nil || len(got) != 0 {
		t.Errorf("no accounts: %+v, %v", got, err)
	}
}

func TestStoreDefault(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if name, err := stores.StoreDefault(ts.Session, ""); err != nil || name != "me" {
		t.Errorf("current = %q, %v", name, err)
	}
	if _, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "org", Prefix: "ORG"}); err != nil {
		t.Fatal(err)
	}
	_, p, _ := ts.Config()
	// The same name: config.yaml is not written (its time and bytes stay).
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p.ConfigFile, old, old); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p.ConfigFile)
	if name, err := stores.StoreDefault(ts.Session, "me"); err != nil || name != "me" {
		t.Errorf("same = %q, %v", name, err)
	}
	if fi, _ := os.Stat(p.ConfigFile); !fi.ModTime().Equal(old) {
		t.Errorf("modified: %v", fi.ModTime())
	}
	if after, _ := os.ReadFile(p.ConfigFile); string(after) != string(before) {
		t.Error("bytes changed")
	}
	if name, err := stores.StoreDefault(ts.Session, "org"); err != nil || name != "org" {
		t.Errorf("org = %q, %v", name, err)
	}
	if name, _ := stores.StoreDefault(ts.Session, ""); name != "org" {
		t.Errorf("current = %q", name)
	}
	// A name that is no store: not found, with the names.
	_, err := stores.StoreDefault(ts.Session, "nope")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "store.not_found")
	if !strings.Contains(err.Error(), "me, org") {
		t.Errorf("message = %v", err)
	}
	if e := usecase.AsError(err); e.Args["name"] != "nope" || len(e.Args["candidates"].([]string)) != 2 {
		t.Errorf("args = %v", e.Args)
	}
}

func TestStoreDefaultNoStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := stores.StoreRemove(ts.Session, "me", true); err != nil {
		t.Fatal(err)
	}
	if name, err := stores.StoreDefault(ts.Session, ""); err != nil || name != "" {
		t.Errorf("current = %q, %v", name, err)
	}
	_, err := stores.StoreDefault(ts.Session, "me")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "store.not_found")
}
