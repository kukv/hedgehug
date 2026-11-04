package usecase

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/auth"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/osfs"
	"go.yaml.in/yaml/v4"
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

// storeAccountRE matches the account: alice line of a store in config.yaml,
// which store init writes.
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
	if !strings.Contains(string(b), defaultAccountYAML) {
		ts.t.Fatalf("config.yaml has no default account to drop:\n%s", b)
	}
	text := storeAccountRE.ReplaceAllString(strings.Replace(string(b), defaultAccountYAML, "", 1), "")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}

// nameAccountOfStores gives the stores that have no account the account name,
// the first of the accounts that replaced the default one: with two or more
// accounts a store without one does not load.
func (ts *testSession) nameAccountOfStores(name string) {
	ts.t.Helper()
	p := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Load(b, &doc); err != nil {
		ts.t.Fatal(err)
	}
	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "stores" {
			continue
		}
		for _, st := range root.Content[i+1].Content {
			named := false
			for j := 0; j+1 < len(st.Content); j += 2 {
				named = named || st.Content[j].Value == "account"
			}
			if !named {
				st.Content = append(st.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "account"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name})
			}
		}
	}
	out, err := yaml.Dump(&doc, yaml.WithLineWidth(-1), yaml.WithIndent(2))
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := os.WriteFile(p, out, 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}

// firstAccountName is the name: of the first account in the accounts: text.
func firstAccountName(t *testing.T, text string) string {
	t.Helper()
	i := strings.Index(text, "accounts:")
	if i < 0 {
		t.Fatal("no accounts: in the config text")
	}
	m := regexp.MustCompile(`name:\s*"?([\w-]+)`).FindStringSubmatch(text[i:])
	if m == nil {
		return ""
	}
	return m[1]
}

// dropAccounts removes the accounts: section from config.yaml.
func (ts *testSession) dropAccounts() {
	ts.t.Helper()
	p := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Load(b, &doc); err != nil {
		ts.t.Fatal(err)
	}
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == "accounts" {
				root.Content = append(root.Content[:i], root.Content[i+2:]...)
				break
			}
		}
	}
	out, err := yaml.Dump(&doc, yaml.WithLineWidth(-1), yaml.WithIndent(2))
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := os.WriteFile(p, out, 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}

// BeforeCommand is Session.BeforeCommand for the config as it is now.
func (ts *testSession) BeforeCommand(start func(s *store.Store, name string) error, report func(store, msg string)) {
	ts.t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	ts.Session.BeforeCommand(c, start, report)
}

// hasDefaultAccount reports whether config.yaml still has defaultAccountYAML.
func (ts *testSession) hasDefaultAccount() bool {
	ts.t.Helper()
	b, err := os.ReadFile(filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")) //nolint:gosec // the config of the test
	if err != nil {
		return false
	}
	return strings.Contains(string(b), defaultAccountYAML)
}

// appendConfig adds text to config.yaml; text that has accounts: replaces the
// default account, if it is still there.
func (ts *testSession) appendConfig(text string) {
	ts.t.Helper()
	if strings.Contains(text, "accounts:") {
		if ts.hasDefaultAccount() {
			ts.dropDefaultAccount()
		}
		if strings.Count(text, "login:") > 1 {
			defer ts.nameAccountOfStores(firstAccountName(ts.t, text))
		}
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
	author := config.CommitAuthor(testAccount)
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
		author := config.CommitAuthor(acct)
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

// addItem puts it into the inbox of the test store, as a sync would, and
// returns its ULID.
func (ts *testSession) addItem(it inbox.Item) model.ULID {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	added, err := inbox.Open(s).Add([]inbox.Item{it})
	if err != nil || len(added) != 1 {
		ts.t.Fatalf("Add item = %v, %v", added, err)
	}
	return added[0].ULID
}

// overwriteME1 makes the store of ts overwrite a change of ME-1's title: ts
// and a clone of it set the title to different values, the clone syncs first,
// then ts, whose own value loses the merge. It returns the inbox item that
// tells of it, and the title ME-1 has after.
func overwriteME1(t *testing.T, ts *testSession) (item InboxItemView, kept string) {
	t.Helper()
	ts.useAccount()
	remote := addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	// The clone writes later than ts, so its title wins the merge.
	other := cloneSession(t, ts, remote, ts.Now().Add(time.Hour))
	other.appendConfig(bobAccount)
	if _, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "mine"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "theirs"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	_, items, err := ts.InboxList(false)
	if err != nil || len(items) != 1 {
		t.Fatalf("InboxList = %v, %v; want 1 overwritten item", items, err)
	}
	_, ix, _ := ts.Open()
	e, err := ts.Resolve(ix, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	return items[0], e.Title
}

func (ts *testSession) viewFile(name string) string {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Dir(), "views", name+".yaml"))
	if err != nil {
		ts.t.Fatal(err)
	}
	return string(b)
}

func (ts *testSession) headSubject() string {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	es, err := logOf(s.Repo(), 1)
	if err != nil || len(es) == 0 {
		ts.t.Fatalf("log: %v", err)
	}
	return es[0].Subject
}

func (ts *testSession) headTrailer(key string) (string, bool) {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	es, err := logOf(s.Repo(), 1)
	if err != nil || len(es) == 0 {
		ts.t.Fatalf("log: %v", err)
	}
	return es[0].Trailer(key)
}

// authSession is a session whose github.com is a fakegh, with a keychain in
// memory; accounts is added to config.yaml.
func authSession(t *testing.T, st fakegh.State, accounts string) (*testSession, *kr) {
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
	if accounts == "" {
		ts.dropDefaultAccount()
	}
	ts.appendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\", client_id: Iv23test}\n" + accounts)
	return ts, k
}

func (ts *testSession) lockFile() string {
	ts.t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	return filepath.Join(filepath.Dir(p.ConfigFile), "auth.lock")
}

func (ts *testSession) accounts() []config.Account {
	ts.t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	return c.Accounts
}

// heldElsewhere reports whether another holder has the lock.
func heldElsewhere(lock string) bool {
	err := osfs.WithFileLock(lock, 20*time.Millisecond, func() error { return nil })
	return errors.Is(err, store.ErrLocked)
}

// commitRawFile writes body to the file rel of the test store and commits it
// past the store's checks, as a hand edit or a sync could.
func (ts *testSession) commitRawFile(rel, body string) {
	ts.t.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		ts.t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	p := filepath.Join(e.Path, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		ts.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	author := config.CommitAuthor(testAccount)
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		ts.t.Fatal(err)
	}
}

// editRawFile replaces old by repl in the file rel of the test store and
// commits it.
func (ts *testSession) editRawFile(rel, old, repl string) {
	ts.t.Helper()
	_, e, _ := ts.Entry()
	b, err := os.ReadFile(filepath.Join(e.Path, filepath.FromSlash(rel))) //nolint:gosec // a file of the test's own store
	if err != nil {
		ts.t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		ts.t.Fatalf("%s has no %q", rel, old)
	}
	ts.commitRawFile(rel, strings.Replace(string(b), old, repl, 1))
}

func (ts *testSession) head() string {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	h, err := s.Repo().Head()
	if err != nil {
		ts.t.Fatal(err)
	}
	return h.String()
}

// doctorOne is Doctor of a session with one store: its only report.
func (ts *testSession) doctorOne(fix bool) (DoctorReport, error) {
	rs, err := ts.Doctor(fix)
	if err != nil || len(rs) != 1 {
		return DoctorReport{}, fmt.Errorf("%d reports, %w", len(rs), err)
	}
	return rs[0], nil
}

// putSprintRelease writes a sprint and a release into the workspace ws of the
// test store and returns their ULIDs.
func (ts *testSession) putSprintRelease(ws model.ULID, sprint, release string) (model.ULID, model.ULID) {
	ts.t.Helper()
	s, _, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: sprint}
	r := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: release}
	if _, err := ts.Update(s, store.Op{Name: "sprint"}, func(tx *store.Tx) error {
		return errors.Join(tx.PutSprint(sp), tx.PutRelease(r))
	}); err != nil {
		ts.t.Fatal(err)
	}
	return sp.ULID, r.ULID
}

// workspaceULID returns the ULID of the workspace name.
func (ts *testSession) workspaceULID(name string) model.ULID {
	ts.t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	for _, w := range ix.Workspaces() {
		if w.Name == name {
			return w.ULID
		}
	}
	ts.t.Fatalf("no workspace %s", name)
	return model.ULID{}
}

// order returns the ids of the live tasks under parent in the workspace ws.
func (ts *testSession) order(ws model.ULID, parent *model.ULID) []string {
	ts.t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	got := []string{}
	for _, e := range Siblings(ix, parent, ws, model.ULID{}) {
		got = append(got, e.ID)
	}
	return got
}

// addedTask is the task id names, as the store has it.
func (ts *testSession) addedTask(id string) *model.Task {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	r, err := ts.Show(id)
	if err != nil {
		ts.t.Fatal(err)
	}
	tk, err := s.Task(r.Entry.ULID)
	if err != nil {
		ts.t.Fatal(err)
	}
	return tk
}

// asAccount makes login the only account of config.yaml, which has its
// accounts as the last key.
func (ts *testSession) asAccount(login string) {
	ts.t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.t.Fatal(err)
	}
	text, _, _ := strings.Cut("\n"+string(b), "\naccounts:")
	text = strings.TrimPrefix(text, "\n")
	if text != "" {
		text += "\n"
	}
	text += "accounts:\n  - {name: " + login + ", host: github.com, login: " + login + "}\n"
	if err := os.WriteFile(p.ConfigFile, []byte(text), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}

func (ts *testSession) setMembers(m replicate.Members) {
	ts.t.Helper()
	s, _, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := replicate.WriteMembers(s, m); err != nil {
		ts.t.Fatal(err)
	}
}

// addProject adds a task of kind project and returns its ULID.
func (ts *testSession) addProject(title string) model.ULID {
	ts.t.Helper()
	r, err := ts.Add(AddInput{Title: title, Kind: "project"})
	if err != nil {
		ts.t.Fatal(err)
	}
	return r.ULID
}

// storeSchema is schema.yaml of the test store, read from the file.
func (ts *testSession) storeSchema() (*schema.Schema, string) {
	ts.t.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(e.Path, "schema.yaml")) //nolint:gosec // a file of the test's own store
	if err != nil {
		ts.t.Fatal(err)
	}
	sc, err := schema.Decode(b)
	if err != nil {
		ts.t.Fatal(err)
	}
	return sc, string(b)
}

// addDeleted writes a task that was deleted at.
func (ts *testSession) addDeleted(title string, at time.Time, edit func(*model.Task)) *model.Task {
	ts.t.Helper()
	return ts.addTask(title, func(t *model.Task) {
		t.DeletedAt = &at
		if edit != nil {
			edit(t)
		}
	})
}

func lastOp(t *testing.T, ts *testSession) string {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().LogFrom(head, 1)
	if err != nil || len(log) == 0 {
		t.Fatalf("log = %v, %v", log, err)
	}
	op, _ := log[0].Trailer("Hedgehug-Op")
	return op
}

// addWorkspace adds a workspace to the test store.
func (ts *testSession) addWorkspace(name, prefix string) {
	ts.t.Helper()
	s, _, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	w := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
	if _, err := ts.Update(s, store.Op{Name: "workspace", Workspace: w.ULID}, func(tx *store.Tx) error { return tx.PutWorkspace(w) }); err != nil {
		ts.t.Fatal(err)
	}
}

// deleteTask sets DeletedAt of the task id.
func (ts *testSession) deleteTask(id string) {
	ts.t.Helper()
	s, ix, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	e, err := ts.Resolve(ix, id)
	if err != nil {
		ts.t.Fatal(err)
	}
	now := ts.Now()
	if _, err := ts.Update(s, store.Op{Name: "rm", Task: e.ULID}, func(tx *store.Tx) error {
		t2, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		t2.DeletedAt = &now
		return tx.PutTask(t2)
	}); err != nil {
		ts.t.Fatal(err)
	}
}

// useAccount makes github.com/alice the account of config.yaml.
func (ts *testSession) useAccount() {
	ts.t.Helper()
	ts.appendConfig("accounts:\n  - name: alice\n    host: github.com\n    login: alice\n")
}

// wantErr checks that err has the kind and the code.
func wantErr(t *testing.T, err error, kind Kind, code string) {
	t.Helper()
	e := AsError(err)
	if err == nil {
		t.Fatalf("no error; want %v %q", kind, code)
	}
	if KindOf(err) != kind {
		t.Errorf("KindOf(%v) = %v; want %v", err, KindOf(err), kind)
	}
	if code != "" && e.Code != code {
		t.Errorf("Code of %v = %+v; want %q", err, e, code)
	}
}

func asError(err error, target **Error) bool { return errors.As(err, target) }

const bobAccount = "\naccounts:\n  - name: bob\n    host: github.com\n    login: bob\n"

// asWriter makes the accounts of config.yaml those of from's replacement to,
// and returns a session of its own, which has not read the writer yet.
func (ts *testSession) asWriter(from, to string) *Session {
	ts.t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigFile, []byte(strings.Replace(string(b), from, to, 1)), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	return &Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn}
}

// moveFx is org-a (API) with a project API-2, its child API-3 and that
// one's child API-4, and API-5 outside; API-3 has a comment and a worklog
// entry; API-4 is blocked by API-1 and API-5 by API-3. org-b is WEB.
type moveFx struct {
	ts                     *testSession
	p, c, g, o, first      model.ULID
	comment, worklog       model.ULID
	commentRel, worklogRel string
}

func newMoveFx(t *testing.T) *moveFx {
	t.Helper()
	ts := newTwoStoreSession(t)
	a := ts.ForStore("org-a")
	add := func(title, parent, kind string) model.ULID {
		r, err := a.Add(AddInput{Title: title, Parent: parent, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		return r.ULID
	}
	f := &moveFx{ts: ts}
	f.p = add("P", "", "project")
	f.c = add("C", "API-2", "")
	f.g = add("G", "API-3", "")
	f.o = add("O", "", "")
	ix := mustIndex(t, ts, "org-a")
	e, err := a.Resolve(ix, "API-1")
	if err != nil {
		t.Fatal(err)
	}
	f.first = e.ULID
	if _, err := a.Dep("API-4", "API-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Dep("API-5", "API-3", true); err != nil {
		t.Fatal(err)
	}
	f.comment, f.worklog = model.NewULID(), model.NewULID()
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Update(s, store.Op{Name: "comment", Task: f.c}, func(tx *store.Tx) error {
		if err := tx.PutComment(f.c, &model.Comment{ULID: f.comment, Author: "github.com/alice", Body: "first"}); err != nil {
			return err
		}
		return tx.PutWorklog(f.c, &model.Worklog{ULID: f.worklog, Author: "github.com/alice", Duration: 3600, Body: "work"})
	})
	if err != nil {
		t.Fatal(err)
	}
	f.commentRel, f.worklogRel = model.CommentPath(f.c, f.comment), model.WorklogPath(f.c, f.worklog)
	return f
}

func (f *moveFx) in() MoveStoreInput {
	return MoveStoreInput{Arg: "API-2", To: "org-b", Yes: true}
}

func (f *moveFx) head(t *testing.T, name string) string {
	t.Helper()
	s, err := f.ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

func (f *moveFx) log(t *testing.T, name string) []gitrepo.LogEntry {
	t.Helper()
	s, err := f.ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	l, err := logOf(s.Repo(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// setSchema changes the schema of org-a.
func (f *moveFx) setSchema(t *testing.T, fn func(*schema.Schema)) {
	t.Helper()
	ss := f.ts.ForStore("org-a")
	s, err := ss.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	sc, err := cloneSchemaOf(s.Schema())
	if err != nil {
		t.Fatal(err)
	}
	fn(sc)
	if _, err := ss.Update(s, store.Op{Name: "schema"}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
}

func mustCode(t *testing.T, err error, kind Kind, code string) {
	t.Helper()
	if e := AsError(err); err == nil || e.Kind != kind || e.Code != code {
		t.Fatalf("err = %v; want kind %v code %s", err, kind, code)
	}
}

const aliceAccount = "\naccounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

// movedFixture moves org-a's API-1 to org-b: org-b gets the task as WEB-2
// (API-1 as a former id), and org-a's file becomes a forward.
// It returns the ULID of the task.
func movedFixture(t *testing.T, ts *testSession) model.ULID {
	t.Helper()
	sa, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	sb, ix, err := ts.ForStore("org-b").Open()
	if err != nil {
		t.Fatal(err)
	}
	ea, err := ts.ForStore("org-a").Resolve(mustIndex(t, ts, "org-a"), "API-1")
	if err != nil {
		t.Fatal(err)
	}
	ulid := ea.ULID
	nt := &model.Task{ULID: ulid, Title: "moved", Kind: "task", Status: "todo", Workspace: ix.Workspaces()[0].ULID, FormerIDs: []string{"API-1"}}
	if _, err := ts.ForStore("org-b").UpdateCommit(sb, store.Op{Name: "move-in", Task: ulid}, func(tx *store.Tx) error { return tx.PutTask(nt) }); err != nil {
		t.Fatal(err)
	}
	fwd := "---\nulid: " + ulid.String() + "\nmoved_to: " + sb.Meta().ULID.String() + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n"
	if _, err := sa.Update(store.Op{Name: "move-out", Author: movedAuthor, Detail: "API-1 org-a task"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(ulid), []byte(fwd))
	}); err != nil {
		t.Fatal(err)
	}
	return ulid
}

func mustIndex(t *testing.T, ts *testSession, name string) *index.Index {
	t.Helper()
	_, ix, err := ts.ForStore(name).Open()
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func headOf(t *testing.T, s *store.Store) string {
	t.Helper()
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

// ulidOf is the ULID of the task arg names.
func ulidOf(t *testing.T, ts *testSession, arg string) model.ULID {
	t.Helper()
	r, err := ts.Show(arg)
	if err != nil {
		t.Fatal(err)
	}
	return r.Entry.ULID
}

func wantCode(t *testing.T, what string, err error, kind Kind, code string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: no error; want %v %s", what, kind, code)
		return
	}
	if e := AsError(err); e.Kind != kind || e.Code != code {
		t.Errorf("%s: %v is %v %q; want %v %q", what, err, e.Kind, e.Code, kind, code)
	}
}

func makeStoreReadOnly(t *testing.T, ts *testSession, name string) {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := replicate.WriteAccess(s, replicate.Access{ReadOnly: true, Reason: "test", Checked: ts.Now()}); err != nil {
		t.Fatal(err)
	}
}

func openSet(t *testing.T, es ...config.StoreEntry) *mount.Set {
	t.Helper()
	set := mount.Open(&config.Config{Stores: es}, mount.Options{})
	return set
}

// testSchema is the default schema with a property of each other type.
func testSchema() *schema.Schema {
	sc := schema.Default()
	sc.Props = append(sc.Props,
		schema.Prop{Name: "flag", Type: schema.TypeBool},
		schema.Prop{Name: "at", Type: schema.TypeDatetime},
		schema.Prop{Name: "note", Type: schema.TypeText},
		schema.Prop{Name: "tags", Type: schema.TypeMultiEnum, Values: []string{"a", "b"}},
	)
	return sc
}

func mustAssigns(t *testing.T, args ...string) []Assign {
	t.Helper()
	as, err := ParseAssigns(args)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func mustULID(t *testing.T, s string) model.ULID {
	t.Helper()
	u, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

const useGoneWarning = "the project of hedgehug use is not found (hedgehug use --clear removes it)"

// addProjectIn adds a project to the store name and returns its id.
func addProjectIn(t *testing.T, ts *testSession, name, title string) string {
	t.Helper()
	r, err := ts.ForStore(name).Add(AddInput{Title: title, Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Index.Refresh(); err != nil {
		t.Fatal(err)
	}
	e, ok := r.Index.Task(r.ULID)
	if !ok {
		t.Fatalf("the project %s is not in the index", r.ULID)
	}
	return e.ID
}

func loadContext(t *testing.T, ts *testSession) current.State {
	t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	return current.Load(p.DataDir, func(m string) { t.Errorf("context.json warned: %s", m) })
}

// fakeSession is a session whose github.com is a fakegh; accounts is added to
// config.yaml. HH_TOKEN is the token of alice.
func fakeSession(t *testing.T, st fakegh.State, accounts string) (*testSession, *fakegh.Server) {
	t.Helper()
	zero := 0
	st.DeviceInterval = &zero
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	fake := fakegh.New(st)
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "t-alice"
	if accounts == "" {
		ts.dropDefaultAccount()
	}
	ts.appendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n" + accounts)
	return ts, fake
}

func configText(t *testing.T, ts *testSession) string {
	t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func notExist(t *testing.T, path, what string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s is left: %v", what, err)
	}
}

func storeDirOf(t *testing.T, ts *testSession, name string) string {
	t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(p.StoresDir, name)
}

// hookConfigWrite makes ts write config.yaml through another Config, right
// before the operation updates it, as another process could.
func hookConfigWrite(t *testing.T, ts *testSession, fn func(*config.Config) error) {
	t.Helper()
	ts.BeforeUpdate = func() {
		c, _, err := ts.Config()
		if err != nil {
			t.Error(err)
			return
		}
		if err := c.Update(fn); err != nil {
			t.Error(err)
		}
		ts.BeforeUpdate = nil
	}
}

// logOf is the first-parent log of HEAD of r, newest first, capped at limit
// when it is above zero.
func logOf(r *gitrepo.Repo, limit int) ([]gitrepo.LogEntry, error) {
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	return r.LogFrom(head, limit)
}

// cfgStore is a store that setConfig writes into config.yaml; account is
// empty for a store with no account: of its own.
type cfgStore struct{ name, account string }

// kr is a keychain in memory. onSet and onDelete run before the call, and
// failSet makes Set fail.
type kr struct {
	mu         sync.Mutex
	m          map[string]string
	onSet      func()
	onDelete   func()
	failSet    error
	failDelete error
}

func newKR() *kr { return &kr{m: map[string]string{}} }

func (k *kr) Get(_, user string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[user]
	if !ok {
		return "", auth.ErrNotFound
	}
	return v, nil
}

func (k *kr) Set(_, user, secret string) error {
	if k.onSet != nil {
		k.onSet()
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failSet != nil {
		return k.failSet
	}
	k.m[user] = secret
	return nil
}

func (k *kr) Delete(_, user string) error {
	if k.onDelete != nil {
		k.onDelete()
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failDelete != nil {
		return k.failDelete
	}
	if _, ok := k.m[user]; !ok {
		return auth.ErrNotFound
	}
	delete(k.m, user)
	return nil
}

func (k *kr) has(user string) (string, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[user]
	return v, ok
}

var movedAuthor = gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}

// makeReadOnly writes access.json of the test store as a read-only store.
func makeReadOnly(t *testing.T, ts *testSession, reason string) {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := replicate.WriteAccess(s, replicate.Access{ReadOnly: true, Reason: reason, Checked: ts.Now()}); err != nil {
		t.Fatal(err)
	}
}

// brokenForward puts a forward file with a key it must not have in org-a.
func (f *moveFx) brokenForward(t *testing.T) model.ULID {
	t.Helper()
	u := model.NewULID()
	a := f.ts.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data := "---\nulid: " + u.String() + "\nmoved_to: " + model.NewULID().String() + "\nmoved_at: 2026-10-31T10:00:00Z\ntitle: keep me\n---\n"
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(u), []byte(data))
	}); err != nil {
		t.Fatal(err)
	}
	return u
}

// oldForward puts a forward file that moved at in org-a.
func (f *moveFx) oldForward(t *testing.T, at time.Time) model.ULID {
	t.Helper()
	u := model.NewULID()
	a := f.ts.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data, err := (&model.Forward{ULID: u, MovedTo: model.NewULID(), MovedAt: at}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(u), data)
	}); err != nil {
		t.Fatal(err)
	}
	return u
}

// addSprint puts the sprint "Sprint 1" on C (API-3) of org-a and returns its
// ULID, as TestMoveStoreUnlinksSprintAndReleases does.
func (f *moveFx) addSprint(t *testing.T) model.ULID {
	t.Helper()
	a := f.ts.ForStore("org-a")
	as, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	var sprint model.ULID
	_, err = a.Update(as, store.Op{Name: "sprint"}, func(tx *store.Tx) error {
		c, err := tx.Task(f.c)
		if err != nil {
			return err
		}
		sp := &model.Sprint{ULID: model.NewULID(), Workspace: c.Workspace, Name: "Sprint 1"}
		if err := tx.PutSprint(sp); err != nil {
			return err
		}
		sprint = sp.ULID
		c.Sprint = &sp.ULID
		return tx.PutTask(c)
	})
	if err != nil {
		t.Fatal(err)
	}
	return sprint
}
