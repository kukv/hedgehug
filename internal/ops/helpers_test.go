package ops

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// addTask writes a task titled title into the workspace of the test store, after
// edit changed it, and returns it as the store has it.
func (ts *testSession) addTask(title string, edit func(*model.Task)) *model.Task {
	ts.t.Helper()
	s, ix, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	task := &model.Task{ULID: model.NewULID(), Title: title, Kind: "task", Status: "todo", Workspace: ix.Workspaces()[0].ULID}
	if edit != nil {
		edit(task)
	}
	if _, err := ts.Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
		ts.t.Fatal(err)
	}
	got, err := s.Task(task.ULID)
	if err != nil {
		ts.t.Fatal(err)
	}
	return got
}

// defaultAccountYAML is the account every test session starts with: the
// commits are alice's.
const defaultAccountYAML = "accounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

// testAccount is the account of defaultAccountYAML.
var testAccount = config.Account{Name: "alice", Host: "github.com", Login: "alice"}

var storeAccountRE = regexp.MustCompile(`(?m)^[ \t]+account: alice\n`)

// appendDefaultAccount adds defaultAccountYAML to the end of config.yaml.
func (ts *testSession) appendDefaultAccount() {
	ts.t.Helper()
	if err := os.MkdirAll(ts.env["HEDGEHUG_HOME"], 0o750); err != nil {
		ts.t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(defaultAccountYAML); err != nil {
		ts.t.Fatal(err)
	}
}

// dropDefaultAccount removes defaultAccountYAML, and the stores' reference to
// it, for the tests that have no account or set up their own.
func (ts *testSession) dropDefaultAccount() {
	ts.t.Helper()
	p := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	text := storeAccountRE.ReplaceAllString(strings.Replace(string(b), defaultAccountYAML, "", 1), "")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}

// appendConfig adds text to config.yaml; text that has accounts: replaces the
// default account.
func (ts *testSession) appendConfig(text string) {
	ts.t.Helper()
	if strings.Contains(text, "accounts:") {
		ts.dropDefaultAccount()
	}
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	f, err := os.OpenFile(p.ConfigFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		ts.t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(text); err != nil {
		ts.t.Fatal(err)
	}
}

// testSession is a Session over a fresh store "me" with one workspace
// "me" (prefix ME), in temporary HOME and HEDGEHUG_HOME directories.
type testSession struct {
	*Session
	t     *testing.T
	env   map[string]string
	warns []string
}

func newTestSession(t *testing.T) *testSession {
	t.Helper()
	d := t.TempDir()
	home := filepath.Join(d, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Alice\n\temail = alice@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := &testSession{t: t, env: map[string]string{
		"HOME":               home,
		"HEDGEHUG_HOME":      filepath.Join(d, "hh"),
		"HEDGEHUG_AUTO_SYNC": "0",
	}}
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	ts.Session = &Session{
		Getenv: func(k string) string { return ts.env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(m string) { ts.warns = append(ts.warns, m) },
	}
	c, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(p.StoresDir, "me")
	author, err := config.CommitAuthor(testAccount, true, ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	ws := &model.Workspace{ULID: model.NewULID(), Name: "me", IDPrefix: "ME"}
	if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "me"}, schema.Default(), ws, author, "", ts.Now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("me", dir, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.appendDefaultAccount()
	return ts
}

// resetWriter forgets the writer the session cached, so that it is read from
// config.yaml again after a test changed the accounts.
func (ss *Session) resetWriter() {
	ss.writerMu.Lock()
	defer ss.writerMu.Unlock()
	ss.writerDone = false
}

// addBareRemote creates a bare repository and makes it the remote of the
// test store, in config.yaml. It returns the path of the remote.
func addBareRemote(t *testing.T, ts *testSession) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "r.git")
	if err := gitrepo.InitBare(remote); err != nil {
		t.Fatal(err)
	}
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Select("me")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("me"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("me", e.Path, []string{remote}); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	return remote
}

// otherSession is a second testSession of the same user (the same HOME) with
// a data directory of its own and no store. Its clock is at now.
func otherSession(t *testing.T, ts *testSession, now time.Time) *testSession {
	t.Helper()
	env := map[string]string{}
	for k, v := range ts.env {
		env[k] = v
	}
	env["HEDGEHUG_HOME"] = filepath.Join(t.TempDir(), "hh2")
	c := &testSession{t: t, env: env}
	c.Session = &Session{
		Getenv: func(k string) string { return c.env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(m string) { c.warns = append(c.warns, m) },
	}
	c.appendDefaultAccount()
	return c
}

// cloneSession is otherSession whose store "me" is cloned from remote with
// StoreAdd.
func cloneSession(t *testing.T, ts *testSession, remote string, now time.Time) *testSession {
	t.Helper()
	c := otherSession(t, ts, now)
	if _, err := c.StoreAdd("me", []string{remote}, "", ""); err != nil {
		t.Fatal(err)
	}
	return c
}

// writeAndCommit writes data to rel in the test store and commits it, as a
// hand edit that was committed would be.
func (ts *testSession) writeAndCommit(rel, data string) {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	if _, err := ts.Update(s, store.Op{Name: "test"}, func(tx *store.Tx) error { return tx.PutFile(rel, []byte(data)) }); err != nil {
		ts.t.Fatal(err)
	}
}

// newTwoStoreSession is a session over the stores org-a (prefix API, the
// account alice) and org-b (prefix WEB, the account bob), with no StoreName.
// Each holds one task: API-1 and WEB-1.
func newTwoStoreSession(t *testing.T) *testSession {
	t.Helper()
	return newTwoStoreSessionWith(t, "API", "WEB")
}

// newTwoStoreSessionWith is newTwoStoreSession with the id prefixes of the
// two stores given.
func newTwoStoreSessionWith(t *testing.T, prefixA, prefixB string) *testSession {
	t.Helper()
	d := t.TempDir()
	home := filepath.Join(d, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Carol\n\temail = carol@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := &testSession{t: t, env: map[string]string{
		"HOME":               home,
		"HEDGEHUG_HOME":      filepath.Join(d, "hh"),
		"HEDGEHUG_AUTO_SYNC": "0",
	}}
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	ts.Session = &Session{
		Getenv: func(k string) string { return ts.env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(m string) { ts.warns = append(ts.warns, m) },
	}
	c, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []config.Account{{Name: "alice", Host: "github.com", Login: "alice"}, {Name: "bob", Host: "github.com", Login: "bob"}} {
		if err := c.AddAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	for _, sp := range []struct{ name, prefix, acct string }{{"org-a", prefixA, "alice"}, {"org-b", prefixB, "bob"}} {
		dir := filepath.Join(p.StoresDir, sp.name)
		acct, _ := c.Account(sp.acct)
		author, err := config.CommitAuthor(acct, true, ts.Getenv)
		if err != nil {
			t.Fatal(err)
		}
		ws := &model.Workspace{ULID: model.NewULID(), Name: sp.name, IDPrefix: sp.prefix}
		if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: sp.name}, schema.Default(), ws, author, "", ts.Now); err != nil {
			t.Fatal(err)
		}
		if err := c.AddStore(sp.name, dir, nil); err != nil {
			t.Fatal(err)
		}
		if err := c.SetStoreAccount(sp.name, sp.acct); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"org-a", "org-b"} {
		if _, err := ts.ForStore(name).Add(AddInput{Title: name + " task"}); err != nil {
			t.Fatal(err)
		}
	}
	return ts
}
