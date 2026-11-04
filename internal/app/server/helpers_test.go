package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

const testToken = "test-token"

var (
	specOnce sync.Once
	specV    validator.Validator
	specErr  error
)

// specValidator is the validator of api/openapi.yaml as committed.
func specValidator(t *testing.T) validator.Validator {
	t.Helper()
	specOnce.Do(func() {
		var b []byte
		if b, specErr = os.ReadFile(filepath.Join("..", "..", "..", "api", "openapi.yaml")); specErr != nil {
			return
		}
		doc, err := libopenapi.NewDocument(b)
		if err != nil {
			specErr = err
			return
		}
		v, errs := validator.NewValidator(doc)
		if len(errs) > 0 {
			specErr = errs[0]
			return
		}
		specV = v
	})
	if specErr != nil {
		t.Fatal(specErr)
	}
	return specV
}

// testServer is a Server over a fresh store "me" with one workspace "me"
// (prefix ME), in temporary HOME and HEDGEHUG_HOME directories, behind an
// httptest server.
type testServer struct {
	t    *testing.T
	srv  *Server
	hs   *httptest.Server
	env  map[string]string
	port int
	log  *bytes.Buffer
}

// newTestServer is newTestServerWith with the options as they are.
func newTestServer(t *testing.T) *testServer { return newTestServerWith(t, nil) }

// newTestServerWith is the test server with opt changed by edit (when not
// nil) before New. It is the body newTestServer had in Task 2, with the
// Options made first.
func newTestServerWith(t *testing.T, edit func(*Options)) *testServer {
	t.Helper()
	d := t.TempDir()
	home := filepath.Join(d, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	ts := &testServer{t: t, log: &bytes.Buffer{}, env: map[string]string{
		"HOME":               home,
		"HEDGEHUG_HOME":      filepath.Join(d, "hh"),
		"HEDGEHUG_AUTO_SYNC": "0",
	}}
	if err := os.MkdirAll(filepath.Join(d, "hh"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "hh", "config.yaml"), []byte(defaultAccountYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := stores.StoreInit(ts.session(), stores.StoreInitInput{Name: "me", Prefix: "ME", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	opt := Options{Getenv: ts.getenv, Now: time.Now, Token: testToken, Version: "test", Log: ts.log}
	if edit != nil {
		edit(&opt)
	}
	ts.srv = New(opt)
	// A poll by hand, or hello before the poller looked, starts builds of
	// history indexes that only Run waits for; one still writing would race
	// the removal of the temporary directory. Cleanups run last first, so
	// this runs after the HTTP server closed and before the removal.
	t.Cleanup(ts.srv.poller.waitBuilds)
	ts.hs = httptest.NewServer(ts.srv.Handler())
	t.Cleanup(ts.hs.Close)
	_, p, err := net.SplitHostPort(ts.hs.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ts.port, _ = strconv.Atoi(p)
	ts.srv.SetPort(ts.port)
	return ts
}

// addRemote makes a bare repository the remote of the store "me" and
// returns its path.
func (ts *testServer) addRemote() string {
	ts.t.Helper()
	remote := filepath.Join(ts.t.TempDir(), "r.git")
	if err := gitrepo.InitBare(remote); err != nil {
		ts.t.Fatal(err)
	}
	c, _, err := ts.session().Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	e, err := c.Select("me")
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := c.RemoveStore("me"); err != nil {
		ts.t.Fatal(err)
	}
	if err := c.AddStore("me", e.Path, []string{remote}); err != nil {
		ts.t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		ts.t.Fatal(err)
	}
	return remote
}

// state is sync.json of the store "me".
func (ts *testServer) state() replicate.State {
	ts.t.Helper()
	s, err := ts.session().OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	st, err := replicate.ReadState(s)
	if err != nil {
		ts.t.Fatal(err)
	}
	return st
}

// remoteHead is the commit main points at in the bare repository remote;
// "" before the first push.
func remoteHead(t *testing.T, remote string) string {
	t.Helper()
	r, err := git.PlainOpen(remote)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		return ""
	}
	return ref.Hash().String()
}

// waitFirstSync waits for the sync the loop makes at its start to end.
func waitFirstSync(t *testing.T, ts *testServer) {
	t.Helper()
	eventually(t, "the sync at start", func() bool { return !ts.state().LastSuccess.IsZero() })
}

func (ts *testServer) getenv(k string) string { return ts.env[k] }

// session is an usecase.Session on the same config, as the CLI would make.
func (ts *testServer) session() *usecase.Session {
	return &usecase.Session{Getenv: ts.getenv, Now: time.Now, Warn: func(string) {}}
}

// reply is what the tests need of a response: its body is read into memory
// and closed.
type reply struct {
	Status int
	Header http.Header
	Body   []byte
}

// raw sends req as it is and returns the response.
func (ts *testServer) raw(req *http.Request) *reply {
	ts.t.Helper()
	resp, err := ts.hs.Client().Do(req)
	if err != nil {
		ts.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.t.Fatal(err)
	}
	return &reply{Status: resp.StatusCode, Header: resp.Header, Body: b}
}

// call sends method path with body as JSON (none when nil) and the token,
// checks the status is want and the response is as api/openapi.yaml says,
// and decodes the body into out (when not nil).
func (ts *testServer) call(method, path string, body any, want int, out any) *reply {
	ts.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			ts.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.hs.URL+path, rd)
	if err != nil {
		ts.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp := ts.raw(req)
	b := resp.Body
	if resp.Status != want {
		ts.t.Fatalf("%s %s: status %d, want %d; body %s", method, path, resp.Status, want, b)
	}
	r := &http.Response{StatusCode: resp.Status, Header: resp.Header, Body: io.NopCloser(bytes.NewReader(b))}
	if ok, errs := specValidator(ts.t).ValidateHttpResponse(req, r); !ok {
		for _, e := range errs {
			ts.t.Errorf("%s %s: response not as api/openapi.yaml says: %s (%s)", method, path, e.Message, e.Reason)
		}
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			ts.t.Fatalf("%s %s: %v; body %s", method, path, err, b)
		}
	}
	return resp
}

// problem calls method path, wants status, and returns the problem.
func (ts *testServer) problem(method, path string, body any, want int) Problem {
	ts.t.Helper()
	var p Problem
	resp := ts.call(method, path, body, want, &p)
	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		ts.t.Errorf("%s %s: Content-Type %q, want application/problem+json", method, path, ct)
	}
	return p
}

func mustULID(t *testing.T, s string) model.ULID {
	t.Helper()
	u, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func slicesEqual[T comparable](a, b []T) bool { return slices.Equal(a, b) }

// testAccount is the account of defaultAccountYAML.
var testAccount = config.Account{Name: "alice", Host: "github.com", Login: "alice"}

// defaultAccountYAML is the account every test server starts with.
const defaultAccountYAML = "accounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

// storeAccountRE matches the account: alice line of a store in config.yaml,
// which store init writes.
var storeAccountRE = regexp.MustCompile(`(?m)^[ \t]+account: alice\n`)

// dropDefaultAccount removes it, and the stores' reference to it, for the
// tests that have no account or set up their own.
func (ts *testServer) dropDefaultAccount() {
	ts.t.Helper()
	p := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	if !strings.Contains(string(b), defaultAccountYAML) {
		ts.t.Fatalf("config.yaml has no default account to drop:\n%s", b)
	}
	text := storeAccountRE.ReplaceAllString(strings.Replace(string(b), defaultAccountYAML, "", 1), "")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		ts.t.Fatal(err)
	}
}

// accountID is the ID of the default account, as the commits
// carry it in Hedgehug-Author.
func (ts *testServer) accountID() string {
	ts.t.Helper()
	c, _, err := ts.session().Config()
	if err != nil || len(c.Accounts) == 0 {
		ts.t.Fatalf("no account: %v", err)
	}
	return c.Accounts[0].ID()
}

// notifyMe puts it into the inbox of the store "me", as a sync would, and
// returns its ULID.
func (ts *testServer) notifyMe(it inbox.Item) string {
	ts.t.Helper()
	return ts.notifyIn("me", it)
}

// notifyIn puts it into the inbox of the named store, as a sync would, and
// returns its ULID.
func (ts *testServer) notifyIn(storeName string, it inbox.Item) string {
	ts.t.Helper()
	ss := ts.session()
	ss.StoreName = storeName
	s, err := ss.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	added, err := inbox.Open(s).Add([]inbox.Item{it})
	if err != nil || len(added) != 1 {
		ts.t.Fatalf("Add item = %v, %v", added, err)
	}
	return added[0].ULID.String()
}

// overwrittenItem is the item a sync makes when it overwrote the title of the
// task (ulid, id) of mine; value is the JSON of my value.
func overwrittenItem(ulid model.ULID, id, value string) inbox.Item {
	return inbox.Item{
		Kind: inbox.Overwritten, Task: ulid, ID: id, Path: model.TaskPath(ulid), Commit: "c1",
		Time:   time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
		Detail: json.RawMessage(`{"field":"title","value":` + value + `}`),
	}
}
