package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/fakegh"
)

// replaceConfigYAML replaces config.yaml with s.
func (te *testEnv) replaceConfigYAML(s string) {
	te.t.Helper()
	if err := os.MkdirAll(te.hhHome, 0o750); err != nil {
		te.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.hhHome, "config.yaml"), []byte(s), 0o600); err != nil {
		te.t.Fatal(err)
	}
}

// dropStoreAccounts takes the account: line out of every store of
// config.yaml, as a config.yaml written by hand has it. The accounts are
// written in the flow style, so no line of them starts with account:.
func (te *testEnv) dropStoreAccounts() {
	te.t.Helper()
	var kept []string
	for _, line := range strings.SplitAfter(te.configText(), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "account: ") {
			kept = append(kept, line)
		}
	}
	te.replaceConfigYAML(strings.Join(kept, ""))
	if strings.Contains(te.configText(), "account:") {
		te.t.Fatalf("config.yaml still has an account: %s", te.configText())
	}
}

// fakeHosts starts a fakegh with alice and bob and returns the hosts: of a
// config.yaml that points github.com at it.
func (te *testEnv) fakeHosts(st fakegh.State) string {
	te.t.Helper()
	zero := 0
	if st.DeviceInterval == nil {
		st.DeviceInterval = &zero
	}
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	srv := httptest.NewServer(fakegh.New(st))
	te.t.Cleanup(srv.Close)
	return "hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\", client_id: Iv23test}\n"
}

// storeAccountIDs is the account of each store in store list, in the order of
// the list.
func (te *testEnv) storeAccountIDs() []string {
	te.t.Helper()
	var list []struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal([]byte(te.ok("store", "list", "--json")), &list); err != nil {
		te.t.Fatal(err)
	}
	ids := make([]string, len(list))
	for i, x := range list {
		ids[i] = x.Account
	}
	return ids
}

func TestAuthAdd(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("")
	if out := te.ok("auth", "add", "personal", "--login", "alice"); out != "Added account personal (github.com/alice)\n" {
		t.Errorf("stdout = %q", out)
	}
	if b := te.configText(); !strings.Contains(b, "name: personal") || !strings.Contains(b, "login: alice") || strings.Contains(b, "token:") {
		t.Errorf("config.yaml = %s", b)
	}
	if out := te.ok("auth", "status"); !strings.Contains(out, "personal") || !strings.Contains(out, "github.com/alice") {
		t.Errorf("status = %q", out)
	}
}

func TestAuthAddKeychainSaysHowToGiveTheToken(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("")
	out := te.ok("auth", "add", "work", "--login", "bob", "--host", "github.com", "--token", "keychain")
	if !strings.Contains(out, "Added account work (github.com/bob)\n") || !strings.Contains(out, "Give it the token with: hedgehug auth login work --with-token\n") {
		t.Errorf("stdout = %q", out)
	}
	if b := te.configText(); !strings.Contains(b, "token: keychain") {
		t.Errorf("config.yaml = %s", b)
	}
}

func TestAuthAddJSON(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("")
	var got struct {
		Name  string `json:"name"`
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	out := te.ok("auth", "add", "work", "--login", "bob", "--token", "env:HH_TOKEN", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Name != "work" || got.ID != "github.com/bob" || got.Token != "env:HH_TOKEN" {
		t.Errorf("add --json = %s, %v", out, err)
	}
	// token is written only when the account has one.
	if out := te.ok("auth", "add", "plain", "--login", "carol", "--json"); strings.Contains(out, "token") {
		t.Errorf("add --json without a token = %s", out)
	}
}

// The stores that had no account: use the only account; once there are two
// they would fail to load, so auth add writes the account to them.
func TestAuthAddSecondAccountSetsStoreAccounts(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("store", "init", "org-a", "--prefix", "OA")
	te.dropStoreAccounts()

	out := te.ok("auth", "add", "work", "--login", "bob")
	if want := "Added account work (github.com/bob)\nSet account personal on stores me, org-a\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if got := strings.Count(te.configText(), "account: personal"); got != 2 {
		t.Errorf("config.yaml has %d account: personal, want 2:\n%s", got, te.configText())
	}
	// config.yaml loads, and each store is still written by alice.
	if ids := te.storeAccountIDs(); len(ids) != 2 || ids[0] != "github.com/alice" || ids[1] != "github.com/alice" {
		t.Errorf("store accounts = %v", ids)
	}
	// One store is a "store", not "stores".
	te.replaceConfigYAML("accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	te.ok("store", "init", "solo", "--prefix", "SO")
	te.dropStoreAccounts()
	if out := te.ok("auth", "add", "work", "--login", "bob"); !strings.Contains(out, "Set account personal on store solo\n") {
		t.Errorf("stdout = %q", out)
	}
}

func TestAuthAddErrors(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	before := te.configText()
	for _, c := range []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"no login", []string{"auth", "add", "work"}, 2, "--login is required"},
		{"no name", []string{"auth", "add", "--login", "bob"}, 2, "accepts 1 arg(s), received 0"},
		{"device", []string{"auth", "add", "work", "--login", "bob", "--token", "device"}, 2, "want gh, env:<VAR> or keychain"},
		{"same name", []string{"auth", "add", "personal", "--login", "bob"}, 1, "set up already"},
		{"same login", []string{"auth", "add", "other", "--login", "Alice"}, 1, "set up already"},
	} {
		code, _, errs := te.run(c.args...)
		if code != c.code || !strings.Contains(errs, c.msg) {
			t.Errorf("%s: exit %d, stderr %q; want exit %d and %q", c.name, code, errs, c.code, c.msg)
		}
	}
	if after := te.configText(); after != before {
		t.Errorf("config.yaml changed by refused adds:\n%s", after)
	}
}

// 直列: keyring.MockInit で go-keyring のモックを作り直す
func TestAuthLoginSecondAccountSetsStoreAccounts(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.replaceConfigYAML(te.fakeHosts(fakegh.State{Tokens: map[string]string{"t-bob": "bob"}}) +
		"accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.dropStoreAccounts()
	te.stdin = "t-bob\n"
	out := te.ok("auth", "login", "work", "--with-token")
	if want := "Logged in as github.com/bob (account work)\nSet account personal on store me\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if ids := te.storeAccountIDs(); len(ids) != 1 || ids[0] != "github.com/alice" {
		t.Errorf("store accounts = %v", ids)
	}
}
