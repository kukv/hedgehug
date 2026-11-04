package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/app/remote/auth"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// memKeyring is a keychain in memory.
type memKeyring struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemKeyring() *memKeyring { return &memKeyring{m: map[string]string{}} }

func (k *memKeyring) Get(_, user string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[user]
	if !ok {
		return "", auth.ErrNotFound
	}
	return v, nil
}

func (k *memKeyring) Set(_, user, secret string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[user] = secret
	return nil
}

func (k *memKeyring) Delete(_, user string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.m[user]; !ok {
		return auth.ErrNotFound
	}
	delete(k.m, user)
	return nil
}

func (k *memKeyring) has(user string) (string, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[user]
	return v, ok
}

// accountsSetup is what newAccountsServer sets up before the server starts.
type accountsSetup struct {
	gh          fakegh.State
	noClientID  bool              // hosts[] with no client_id: the device flow cannot start
	env         map[string]string // the environment of the server (tokens of env:VAR)
	dropDefault bool              // no account at the start (dropDefaultAccount)
}

// newAccountsServer is a started test server whose keychain is in memory and
// whose github.com is a fakegh (in this process, over httptest).
func newAccountsServer(t *testing.T, su accountsSetup) (*testServer, *memKeyring, *fakegh.Server) {
	t.Helper()
	k := newMemKeyring()
	ts := newTestServerWith(t, func(o *Options) { o.Keyring = k })
	if su.dropDefault {
		ts.dropDefaultAccount()
	}
	if su.gh.Users == nil {
		su.gh.Users = []string{"alice", "bob", "carol"}
	}
	fg := fakegh.New(su.gh)
	srv := httptest.NewServer(fg)
	t.Cleanup(srv.Close)
	hosts := "\nhosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\""
	if !su.noClientID {
		hosts += ", client_id: Iv23test"
	}
	hosts += "}\n"
	f, err := os.OpenFile(filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(hosts); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for k, v := range su.env {
		ts.env[k] = v
	}
	ctx, cancel := context.WithCancel(context.Background())
	ts.srv.Start(ctx)
	t.Cleanup(func() { cancel(); _ = ts.srv.Shutdown(context.Background()) })
	return ts, k, fg
}

// noSecret fails when any of the bodies has secret. A token must be in no
// response, no event and no line of the log.
func noSecret(t *testing.T, secret string, bodies ...[]byte) {
	t.Helper()
	if secret == "" {
		t.Fatal("noSecret: no secret to look for")
	}
	for i, b := range bodies {
		if bytes.Contains(b, []byte(secret)) {
			t.Errorf("body %d has the token %q: %s", i, secret, b)
		}
	}
}

// logText is what the server wrote to its log so far.
func (ts *testServer) logText() []byte {
	ts.srv.logMu.Lock()
	defer ts.srv.logMu.Unlock()
	return bytes.Clone(ts.log.Bytes())
}

type accountRow struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	Token string `json:"token"`
	State string `json:"state"`
	Login string `json:"login"`
	Error string `json:"error"`
}

// GET /api/accounts is hh auth status --json; accounts in error are in the
// array, with 200 (ErrFindings is not a failure of the request).
func TestAccountsList(t *testing.T) {
	t.Parallel()
	const secret = "t-bob-secret"
	ts, _, _ := newAccountsServer(t, accountsSetup{
		gh:  fakegh.State{Tokens: map[string]string{secret: "bob"}},
		env: map[string]string{"BOB_TOKEN": secret},
	})
	var bodies [][]byte
	bodies = append(bodies, ts.call("POST", "/api/accounts", map[string]any{"name": "bob", "login": "bob", "token": "env:BOB_TOKEN"}, 201, nil).Body)
	bodies = append(bodies, ts.call("POST", "/api/accounts", map[string]any{"name": "carol", "login": "carol", "token": "env:NOPE"}, 201, nil).Body)

	var list []accountRow
	r := ts.call("GET", "/api/accounts", nil, 200, &list)
	bodies = append(bodies, r.Body)
	if len(list) != 3 {
		t.Fatalf("accounts = %+v; want alice, bob and carol", list)
	}
	want := []accountRow{
		{Name: "alice", ID: "github.com/alice", State: "none"},
		{Name: "bob", ID: "github.com/bob", Token: "env:BOB_TOKEN", State: "ok", Login: "bob"},
		{Name: "carol", ID: "github.com/carol", Token: "env:NOPE", State: "error"},
	}
	for i, w := range want {
		g := list[i]
		if g.Name != w.Name || g.ID != w.ID || g.Token != w.Token || g.State != w.State || g.Login != w.Login {
			t.Errorf("accounts[%d] = %+v; want %+v", i, g, w)
		}
	}
	if list[2].Error == "" {
		t.Errorf("carol has no error text: %+v", list[2])
	}
	noSecret(t, secret, append(bodies, ts.logText())...)
}

func TestAccountsAdd(t *testing.T) {
	t.Parallel()
	ts, _, _ := newAccountsServer(t, accountsSetup{})

	// keychain and device are not for this call: the answer says what is.
	for _, tok := range []string{"keychain", "device"} {
		p := ts.problem("POST", "/api/accounts", map[string]any{"name": "x", "login": "x", "token": tok}, 400)
		if p.Type != "auth.token_source" || p.Args["token"] != tok {
			t.Errorf("token %q: problem = %+v", tok, p)
		}
	}
	var res struct {
		Name  string `json:"name"`
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	ts.call("POST", "/api/accounts", map[string]any{"name": "g", "login": "gee", "token": "gh"}, 201, &res)
	if res.Name != "g" || res.ID != "github.com/gee" || res.Token != "gh" {
		t.Errorf("result = %+v", res)
	}
	res.Token = "" // Unmarshal leaves a field the body omits as it was
	ts.call("POST", "/api/accounts", map[string]any{"name": "n", "login": "nobody", "host": "ghe.example.com"}, 201, &res)
	if res.ID != "ghe.example.com/nobody" || res.Token != "" {
		t.Errorf("no token: %+v", res)
	}
	if p := ts.problem("POST", "/api/accounts", map[string]any{"name": "g", "login": "other"}, 409); p.Type != "account.exists" {
		t.Errorf("same name: %+v", p)
	}
	if p := ts.problem("POST", "/api/accounts", map[string]any{"name": "x", "login": "x", "token": "vault"}, 400); p.Type != "auth.token_source" {
		t.Errorf("unknown source: %+v", p)
	}
	if p := ts.problem("POST", "/api/accounts", map[string]any{"name": "x", "login": " "}, 400); p.Type != "auth.login" {
		t.Errorf("blank login: %+v", p)
	}
}

// Adding a second account gives the stores that had none the first one, in
// the same write; the stores' accounts changed, so the GUI is told.
func TestAccountsAddPinsStores(t *testing.T) {
	t.Parallel()
	ts, _, _ := newAccountsServer(t, accountsSetup{dropDefault: true})
	es := ts.events()
	ts.call("POST", "/api/accounts", map[string]any{"name": "a", "login": "a"}, 201, nil)
	ts.call("POST", "/api/accounts", map[string]any{"name": "b", "login": "b"}, 201, nil)
	es.next(t, "stores.changed")
	var stores []struct {
		Name    string `json:"name"`
		Account string `json:"account"`
	}
	ts.call("GET", "/api/stores", nil, 200, &stores)
	if len(stores) != 1 || stores[0].Account != "github.com/a" {
		t.Errorf("stores = %+v; want me on github.com/a", stores)
	}
}

// PUT /api/accounts/{name}/token is hh auth login <name> --with-token: the
// token must be of the login of the account, and goes to the keychain only.
func TestAccountTokenPut(t *testing.T) {
	t.Parallel()
	const bob, alice = "t-bob-secret", "t-alice-secret"
	ts, k, _ := newAccountsServer(t, accountsSetup{gh: fakegh.State{Tokens: map[string]string{bob: "bob", alice: "alice"}}})
	var bodies [][]byte
	put := func(name string, body map[string]any, want int) (*reply, Problem) {
		t.Helper()
		var p Problem
		var r *reply
		if want == 200 {
			r = ts.call("PUT", "/api/accounts/"+name+"/token", body, want, nil)
		} else {
			r = ts.call("PUT", "/api/accounts/"+name+"/token", body, want, &p)
		}
		bodies = append(bodies, r.Body)
		return r, p
	}

	// a name that is no account's: the account of the token's login is added.
	r, _ := put("bobby", map[string]any{"token": bob}, 200)
	var got struct {
		Account string `json:"account"`
		ID      string `json:"id"`
		Added   bool   `json:"added"`
		Uses    string `json:"uses"`
	}
	if err := json.Unmarshal(r.Body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Account != "bobby" || got.ID != "github.com/bob" || !got.Added || got.Uses != "" {
		t.Errorf("result = %+v", got)
	}
	if v, ok := k.has("github.com/bob"); !ok || v != bob {
		t.Errorf("keychain = %q, %v", v, ok)
	}
	c, _, err := ts.session().Config()
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := c.Account("bobby"); !ok || a.Token.Kind != "keychain" {
		t.Errorf("config account = %+v, %v", a, ok)
	}

	// the token of another login, a token GitHub refuses, and no token.
	if _, p := put("alice", map[string]any{"token": bob}, 422); p.Type != "auth.mismatch" || p.Args["login"] != "bob" {
		t.Errorf("another login: %+v", p)
	}
	if _, p := put("alice", map[string]any{"token": "t-unknown"}, 422); p.Type != "auth.unauthorized" {
		t.Errorf("refused: %+v", p)
	}
	if _, p := put("alice", map[string]any{"token": "  "}, 422); p.Type != "auth.token_empty" {
		t.Errorf("empty: %+v", p)
	}
	if _, ok := k.has("github.com/alice"); ok {
		t.Error("alice has a token in the keychain after the refused ones")
	}
	// the host of an account that exists must be its host.
	if _, p := put("alice", map[string]any{"token": alice, "host": "ghe.example.com"}, 400); p.Type != "auth.host" {
		t.Errorf("another host: %+v", p)
	}

	// the right token of an account that exists.
	r, _ = put("alice", map[string]any{"token": alice}, 200)
	if err := json.Unmarshal(r.Body, &got); err != nil || got.Added || got.Account != "alice" {
		t.Errorf("alice: %s, %v", r.Body, err)
	}
	if v, _ := k.has("github.com/alice"); v != alice {
		t.Errorf("keychain = %q", v)
	}

	// an account of gh has nothing to log in: nothing is written, and it says so.
	ts.call("POST", "/api/accounts", map[string]any{"name": "g", "login": "gee", "token": "gh"}, 201, nil)
	r, _ = put("g", map[string]any{"token": alice}, 200)
	if err := json.Unmarshal(r.Body, &got); err != nil || got.Uses != "gh" || got.Added {
		t.Errorf("gh: %s, %v", r.Body, err)
	}
	if _, ok := k.has("github.com/gee"); ok {
		t.Error("an account of gh has a token in the keychain")
	}

	for _, s := range []string{bob, alice} {
		noSecret(t, s, append(bodies, ts.logText())...)
	}
}

func TestAccountTokenDelete(t *testing.T) {
	t.Parallel()
	const alice = "t-alice-secret"
	ts, k, _ := newAccountsServer(t, accountsSetup{gh: fakegh.State{Tokens: map[string]string{alice: "alice"}}})
	ts.call("PUT", "/api/accounts/alice/token", map[string]any{"token": alice}, 200, nil)
	if _, ok := k.has("github.com/alice"); !ok {
		t.Fatal("no token to remove")
	}
	var res struct {
		Account string `json:"account"`
		ID      string `json:"id"`
		Token   string `json:"token"`
		Removed bool   `json:"removed"`
	}
	r := ts.call("DELETE", "/api/accounts/alice/token", nil, 200, &res)
	if res.Account != "alice" || res.ID != "github.com/alice" || res.Token != "keychain" || !res.Removed {
		t.Errorf("result = %+v", res)
	}
	if _, ok := k.has("github.com/alice"); ok {
		t.Error("the token is in the keychain still")
	}
	noSecret(t, alice, r.Body, ts.logText())

	// gh: Hedgehug keeps nothing, so nothing is removed.
	ts.call("POST", "/api/accounts", map[string]any{"name": "g", "login": "gee", "token": "gh"}, 201, nil)
	ts.call("DELETE", "/api/accounts/g/token", nil, 200, &res)
	if res.Removed || res.Token != "gh" {
		t.Errorf("gh: %+v", res)
	}
	if p := ts.problem("DELETE", "/api/accounts/nobody/token", nil, 404); p.Type != "account.not_found" {
		t.Errorf("unknown: %+v", p)
	}
}
