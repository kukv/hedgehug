// Package usecasetest makes sessions over test stores, for the tests of
// usecase and its areas.
package usecasetest

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
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/osfs"
	"go.yaml.in/yaml/v4"
)

// AddTask writes a task titled title into the workspace of the test store, after
// edit changed it, and returns it as the store has it.
func (ts *Session) AddTask(title string, edit func(*model.Task)) *model.Task {
	ts.T.Helper()
	s, ix, err := ts.Open()
	if err != nil {
		ts.T.Fatal(err)
	}
	task := &model.Task{ULID: model.NewULID(), Title: title, Kind: "task", Status: "todo", Workspace: ix.Workspaces()[0].ULID}
	if edit != nil {
		edit(task)
	}
	if _, err := ts.Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
		ts.T.Fatal(err)
	}
	got, err := s.Task(task.ULID)
	if err != nil {
		ts.T.Fatal(err)
	}
	return got
}

// DefaultAccountYAML is the account every test session starts with: the
// commits are alice's.
const DefaultAccountYAML = "accounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

// TestAccount is the account of DefaultAccountYAML.
var TestAccount = config.Account{Name: "alice", Host: "github.com", Login: "alice"}

// StoreAccountRE matches the account: alice line of a store in config.yaml,
// which store init writes.
var StoreAccountRE = regexp.MustCompile(`(?m)^[ \t]+account: alice\n`)

// AppendDefaultAccount adds DefaultAccountYAML to the end of config.yaml.
func (ts *Session) AppendDefaultAccount() {
	ts.T.Helper()
	if err := os.MkdirAll(ts.Env["HEDGEHUG_HOME"], 0o750); err != nil {
		ts.T.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(ts.Env["HEDGEHUG_HOME"], "config.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the config of the test
	if err != nil {
		ts.T.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(DefaultAccountYAML); err != nil {
		ts.T.Fatal(err)
	}
}

// Renew replaces the session with a new one that has the same outside world
// (Getenv, Now, the hooks and the rest of the exported fields), as a new run
// of the command starts: the writer, the tokens and the stores mounted are
// read again. The helpers that change config.yaml call it; call it after a
// change made by hand, or for a session that has not cached anything.
func (ts *Session) Renew() {
	o := ts.Session
	ts.Session = &usecase.Session{
		Getenv:         o.Getenv,
		Now:            o.Now,
		Loc:            o.Loc,
		StoreName:      o.StoreName,
		Warn:           o.Warn,
		AfterWrite:     o.AfterWrite,
		BackgroundSync: o.BackgroundSync,
		BeforeUpdate:   o.BeforeUpdate,
		BeforeSwap:     o.BeforeSwap,
		Spawn:          o.Spawn,
		Keyring:        o.Keyring,
		HTTPClient:     o.HTTPClient,
		Run:            o.Run,
		Progress:       o.Progress,
	}
}

// DropDefaultAccount removes DefaultAccountYAML, and the stores' reference to
// it, for the tests that have no account or set up their own.
func (ts *Session) DropDefaultAccount() {
	ts.T.Helper()
	p := filepath.Join(ts.Env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.T.Fatal(err)
	}
	if !strings.Contains(string(b), DefaultAccountYAML) {
		ts.T.Fatalf("config.yaml has no default account to drop:\n%s", b)
	}
	text := StoreAccountRE.ReplaceAllString(strings.Replace(string(b), DefaultAccountYAML, "", 1), "")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		ts.T.Fatal(err)
	}
	ts.Renew()
}

// NameAccountOfStores gives the stores that have no account the account name,
// the first of the accounts that replaced the default one: with two or more
// accounts a store without one does not load.
func (ts *Session) NameAccountOfStores(name string) {
	ts.T.Helper()
	p := filepath.Join(ts.Env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.T.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Load(b, &doc); err != nil {
		ts.T.Fatal(err)
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
		ts.T.Fatal(err)
	}
	if err := os.WriteFile(p, out, 0o600); err != nil {
		ts.T.Fatal(err)
	}
	ts.Renew()
}

// FirstAccountName is the name: of the first account in the accounts: text.
func FirstAccountName(t *testing.T, text string) string {
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

// DropAccounts removes the accounts: section from config.yaml.
func (ts *Session) DropAccounts() {
	ts.T.Helper()
	p := filepath.Join(ts.Env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.T.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Load(b, &doc); err != nil {
		ts.T.Fatal(err)
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
		ts.T.Fatal(err)
	}
	if err := os.WriteFile(p, out, 0o600); err != nil {
		ts.T.Fatal(err)
	}
	ts.Renew()
}

// BeforeCommand is replication.BeforeCommand for the session.
func (ts *Session) BeforeCommand(report func(store, msg string)) {
	ts.T.Helper()
	replication.BeforeCommand(ts.Session, report)
}

// SpawnFunc is a Spawn that starts no process: f gets the arguments of the
// child. A Session or a CLI environment takes it as its Spawn.
func SpawnFunc(f func(args []string)) replicate.Spawn {
	return func(_ string, args []string, _ string) error { f(args); return nil }
}

// HasDefaultAccount reports whether config.yaml still has DefaultAccountYAML.
func (ts *Session) HasDefaultAccount() bool {
	ts.T.Helper()
	b, err := os.ReadFile(filepath.Join(ts.Env["HEDGEHUG_HOME"], "config.yaml")) //nolint:gosec // the config of the test
	if err != nil {
		return false
	}
	return strings.Contains(string(b), DefaultAccountYAML)
}

// AppendConfig adds text to config.yaml; text that has accounts: replaces the
// default account, if it is still there.
func (ts *Session) AppendConfig(text string) {
	ts.T.Helper()
	if strings.Contains(text, "accounts:") {
		if ts.HasDefaultAccount() {
			ts.DropDefaultAccount()
		}
		if strings.Count(text, "login:") > 1 {
			defer ts.NameAccountOfStores(FirstAccountName(ts.T, text))
		}
	}
	_, p, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	f, err := os.OpenFile(p.ConfigFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		ts.T.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(text); err != nil {
		ts.T.Fatal(err)
	}
}

// Session is a Session over a fresh store "me" with one workspace
// "me" (prefix ME), in temporary HOME and HEDGEHUG_HOME directories.
type Session struct {
	*usecase.Session
	T     *testing.T
	Env   map[string]string
	Warns []string
}

func New(t *testing.T) *Session {
	t.Helper()
	d := t.TempDir()
	home := filepath.Join(d, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	ts := &Session{T: t, Env: map[string]string{
		"HOME":               home,
		"HEDGEHUG_HOME":      filepath.Join(d, "hh"),
		"HEDGEHUG_AUTO_SYNC": "0",
	}}
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	ts.Session = &usecase.Session{
		Getenv: func(k string) string { return ts.Env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(m string) { ts.Warns = append(ts.Warns, m) },
	}
	c, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(p.StoresDir, "me")
	author := config.CommitAuthor(TestAccount)
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
	ts.AppendDefaultAccount()
	return ts
}

// AddBareRemote creates a bare repository and makes it the remote of the
// test store, in config.yaml. It returns the path of the remote.
func AddBareRemote(t *testing.T, ts *Session) string {
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

// Other is a second Session of the same user (the same HOME) with
// a data directory of its own and no store. Its clock is at now.
func Other(t *testing.T, ts *Session, now time.Time) *Session {
	t.Helper()
	return OtherClock(t, ts, func() time.Time { return now })
}

// OtherClock is Other whose clock is now, read at each use.
func OtherClock(t *testing.T, ts *Session, now func() time.Time) *Session {
	t.Helper()
	env := map[string]string{}
	for k, v := range ts.Env {
		env[k] = v
	}
	env["HEDGEHUG_HOME"] = filepath.Join(t.TempDir(), "hh2")
	c := &Session{T: t, Env: env}
	c.Session = &usecase.Session{
		Getenv: func(k string) string { return c.Env[k] },
		Now:    now,
		Warn:   func(m string) { c.Warns = append(c.Warns, m) },
	}
	c.AppendDefaultAccount()
	return c
}

// Clone is Other whose store "me" is cloned from remote with
// StoreAdd.
func Clone(t *testing.T, ts *Session, remote string, now time.Time) *Session {
	t.Helper()
	c := Other(t, ts, now)
	if _, err := stores.StoreAdd(c.Session, "me", []string{remote}, "", ""); err != nil {
		t.Fatal(err)
	}
	return c
}

// WriteAndCommit writes data to rel in the test store and commits it, as a
// hand edit that was committed would be.
func (ts *Session) WriteAndCommit(rel, data string) {
	ts.T.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.T.Fatal(err)
	}
	if _, err := ts.Update(s, store.Op{Name: "test"}, func(tx *store.Tx) error { return tx.PutFile(rel, []byte(data)) }); err != nil {
		ts.T.Fatal(err)
	}
}

// NewTwoStore is a session over the stores org-a (prefix API, the
// account alice) and org-b (prefix WEB, the account bob), with no StoreName.
// Each holds one task: API-1 and WEB-1.
func NewTwoStore(t *testing.T) *Session {
	t.Helper()
	return NewTwoStoreWith(t, "API", "WEB")
}

// NewTwoStoreWith is NewTwoStore with the id prefixes of the
// two stores given.
func NewTwoStoreWith(t *testing.T, prefixA, prefixB string) *Session {
	t.Helper()
	d := t.TempDir()
	home := filepath.Join(d, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	ts := &Session{T: t, Env: map[string]string{
		"HOME":               home,
		"HEDGEHUG_HOME":      filepath.Join(d, "hh"),
		"HEDGEHUG_AUTO_SYNC": "0",
	}}
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	ts.Session = &usecase.Session{
		Getenv: func(k string) string { return ts.Env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(m string) { ts.Warns = append(ts.Warns, m) },
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
		if _, err := tasks.Add(ts.ForStore(name), tasks.AddInput{Title: name + " task"}); err != nil {
			t.Fatal(err)
		}
	}
	return ts
}

// SetConfig replaces config.yaml with head (accounts:, hosts:, ...) and the
// stores. The store "me" is the one of the test session; any other is only an
// entry, with a directory that does not exist.
func (ts *Session) SetConfig(head string, stores ...CfgStore) {
	ts.T.Helper()
	c, p, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	me, err := c.Select("me")
	if err != nil {
		ts.T.Fatal(err)
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString("stores:\n")
	for _, s := range stores {
		path := me.Path
		if s.Name != "me" {
			path = filepath.Join(ts.T.TempDir(), s.Name)
		}
		fmt.Fprintf(&b, "  - {name: %s, path: %q", s.Name, path)
		if s.Account != "" {
			fmt.Fprintf(&b, ", account: %s", s.Account)
		}
		b.WriteString("}\n")
	}
	if err := os.WriteFile(p.ConfigFile, []byte(b.String()), 0o600); err != nil {
		ts.T.Fatal(err)
	}
}

// StoreAccounts is the account: of each store in config.yaml, by store name.
func (ts *Session) StoreAccounts() map[string]string {
	ts.T.Helper()
	c, _, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	m := map[string]string{}
	for _, e := range c.Stores {
		m[e.Name] = e.Account
	}
	return m
}

// ConfigBytes is config.yaml as it is on disk.
func (ts *Session) ConfigBytes() []byte {
	ts.T.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.T.Fatal(err)
	}
	return b
}

// AddItem puts it into the inbox of the test store, as a sync would, and
// returns its ULID.
func (ts *Session) AddItem(it inbox.Item) model.ULID {
	ts.T.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.T.Fatal(err)
	}
	added, err := inbox.Open(s).Add([]inbox.Item{it})
	if err != nil || len(added) != 1 {
		ts.T.Fatalf("Add item = %v, %v", added, err)
	}
	return added[0].ULID
}

// OverwriteME1 makes the store of ts overwrite a change of ME-1's title: ts
// and a clone of it set the title to different values, the clone syncs first,
// then ts, whose own value loses the merge. It returns the inbox item that
// tells of it, and the title ME-1 has after.
func OverwriteME1(t *testing.T, ts *Session) (item notifications.InboxItemView, kept string) {
	t.Helper()
	ts.UseAccount()
	remote := AddBareRemote(t, ts)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	// The clone writes later than ts, so its title wins the merge.
	other := Clone(t, ts, remote, ts.Now().Add(time.Hour))
	other.AppendConfig(BobAccount)
	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "mine"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(other.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "theirs"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(other.Session, false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	_, items, err := notifications.InboxList(ts.Session, false)
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

func (ts *Session) ViewFile(name string) string {
	ts.T.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.T.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Dir(), "views", name+".yaml"))
	if err != nil {
		ts.T.Fatal(err)
	}
	return string(b)
}

func (ts *Session) HeadSubject() string {
	ts.T.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.T.Fatal(err)
	}
	es, err := LogOf(s.Repo(), 1)
	if err != nil || len(es) == 0 {
		ts.T.Fatalf("log: %v", err)
	}
	return es[0].Subject
}

func (ts *Session) HeadTrailer(key string) (string, bool) {
	ts.T.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.T.Fatal(err)
	}
	es, err := LogOf(s.Repo(), 1)
	if err != nil || len(es) == 0 {
		ts.T.Fatalf("log: %v", err)
	}
	return es[0].Trailer(key)
}

// AuthSession is a session whose github.com is a fakegh, with a keychain in
// memory; accounts is added to config.yaml.
func AuthSession(t *testing.T, st fakegh.State, accounts string) (*Session, *Keyring) {
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
	ts := New(t)
	k := NewKeyring()
	ts.Keyring = k
	if accounts == "" {
		ts.DropDefaultAccount()
	}
	ts.AppendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\", client_id: Iv23test}\n" + accounts)
	return ts, k
}

func (ts *Session) LockFile() string {
	ts.T.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	return filepath.Join(filepath.Dir(p.ConfigFile), "auth.lock")
}

func (ts *Session) Accounts() []config.Account {
	ts.T.Helper()
	c, _, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	return c.Accounts
}

// HeldElsewhere reports whether another holder has the lock.
func HeldElsewhere(lock string) bool {
	err := osfs.WithFileLock(lock, 20*time.Millisecond, func() error { return nil })
	return errors.Is(err, store.ErrLocked)
}

// CommitRawFile writes body to the file rel of the test store and commits it
// past the store's checks, as a hand edit or a sync could.
func (ts *Session) CommitRawFile(rel, body string) {
	ts.T.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		ts.T.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		ts.T.Fatal(err)
	}
	p := filepath.Join(e.Path, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		ts.T.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		ts.T.Fatal(err)
	}
	author := config.CommitAuthor(TestAccount)
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		ts.T.Fatal(err)
	}
}

// EditRawFile replaces old by repl in the file rel of the test store and
// commits it.
func (ts *Session) EditRawFile(rel, old, repl string) {
	ts.T.Helper()
	_, e, _ := ts.Entry()
	b, err := os.ReadFile(filepath.Join(e.Path, filepath.FromSlash(rel))) //nolint:gosec // a file of the test's own store
	if err != nil {
		ts.T.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		ts.T.Fatalf("%s has no %q", rel, old)
	}
	ts.CommitRawFile(rel, strings.Replace(string(b), old, repl, 1))
}

func (ts *Session) Head() string {
	ts.T.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.T.Fatal(err)
	}
	h, err := s.Repo().Head()
	if err != nil {
		ts.T.Fatal(err)
	}
	return h.String()
}

// DoctorOne is Doctor of a session with one store: its only report.
func (ts *Session) DoctorOne(fix bool) (maintenance.DoctorReport, error) {
	rs, err := maintenance.Doctor(ts.Session, fix)
	if err != nil || len(rs) != 1 {
		return maintenance.DoctorReport{}, fmt.Errorf("%d reports, %w", len(rs), err)
	}
	return rs[0], nil
}

// PutSprintRelease writes a sprint and a release into the workspace ws of the
// test store and returns their ULIDs.
func (ts *Session) PutSprintRelease(ws model.ULID, sprint, release string) (model.ULID, model.ULID) {
	ts.T.Helper()
	s, _, err := ts.Open()
	if err != nil {
		ts.T.Fatal(err)
	}
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: sprint}
	r := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: release}
	if _, err := ts.Update(s, store.Op{Name: "sprint"}, func(tx *store.Tx) error {
		return errors.Join(tx.PutSprint(sp), tx.PutRelease(r))
	}); err != nil {
		ts.T.Fatal(err)
	}
	return sp.ULID, r.ULID
}

// WorkspaceULID returns the ULID of the workspace name.
func (ts *Session) WorkspaceULID(name string) model.ULID {
	ts.T.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		ts.T.Fatal(err)
	}
	for _, w := range ix.Workspaces() {
		if w.Name == name {
			return w.ULID
		}
	}
	ts.T.Fatalf("no workspace %s", name)
	return model.ULID{}
}

// Order returns the ids of the live tasks under parent in the workspace ws.
func (ts *Session) Order(ws model.ULID, parent *model.ULID) []string {
	ts.T.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		ts.T.Fatal(err)
	}
	got := []string{}
	for _, e := range usecase.Siblings(ix, parent, ws, model.ULID{}) {
		got = append(got, e.ID)
	}
	return got
}

// AddedTask is the task id names, as the store has it.
func (ts *Session) AddedTask(id string) *model.Task {
	ts.T.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.T.Fatal(err)
	}
	r, err := browse.Show(ts.Session, id)
	if err != nil {
		ts.T.Fatal(err)
	}
	u, err := model.ParseULID(r.ULID)
	if err != nil {
		ts.T.Fatal(err)
	}
	tk, err := s.Task(u)
	if err != nil {
		ts.T.Fatal(err)
	}
	return tk
}

// AsAccount makes login the only account of config.yaml, which has its
// accounts as the last key.
func (ts *Session) AsAccount(login string) {
	ts.T.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.T.Fatal(err)
	}
	text, _, _ := strings.Cut("\n"+string(b), "\naccounts:")
	text = strings.TrimPrefix(text, "\n")
	if text != "" {
		text += "\n"
	}
	text += "accounts:\n  - {name: " + login + ", host: github.com, login: " + login + "}\n"
	if err := os.WriteFile(p.ConfigFile, []byte(text), 0o600); err != nil {
		ts.T.Fatal(err)
	}
	ts.Renew()
}

func (ts *Session) SetMembers(m replicate.Members) {
	ts.T.Helper()
	s, _, err := ts.Open()
	if err != nil {
		ts.T.Fatal(err)
	}
	if err := replicate.WriteMembers(s, m); err != nil {
		ts.T.Fatal(err)
	}
}

// AddProject adds a task of kind project and returns its ULID.
func (ts *Session) AddProject(title string) model.ULID {
	ts.T.Helper()
	r, err := tasks.Add(ts.Session, tasks.AddInput{Title: title, Kind: "project"})
	if err != nil {
		ts.T.Fatal(err)
	}
	return r.ULID
}

// StoreSchema is schema.yaml of the test store, read from the file.
func (ts *Session) StoreSchema() (*schema.Schema, string) {
	ts.T.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		ts.T.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(e.Path, "schema.yaml")) //nolint:gosec // a file of the test's own store
	if err != nil {
		ts.T.Fatal(err)
	}
	sc, err := schema.Decode(b)
	if err != nil {
		ts.T.Fatal(err)
	}
	return sc, string(b)
}

// AddDeleted writes a task that was deleted at.
func (ts *Session) AddDeleted(title string, at time.Time, edit func(*model.Task)) *model.Task {
	ts.T.Helper()
	return ts.AddTask(title, func(t *model.Task) {
		t.DeletedAt = &at
		if edit != nil {
			edit(t)
		}
	})
}

func LastOp(t *testing.T, ts *Session) string {
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

// AddWorkspace adds a workspace to the test store.
func (ts *Session) AddWorkspace(name, prefix string) {
	ts.T.Helper()
	s, _, err := ts.Open()
	if err != nil {
		ts.T.Fatal(err)
	}
	w := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
	if _, err := ts.Update(s, store.Op{Name: "workspace", Workspace: w.ULID}, func(tx *store.Tx) error { return tx.PutWorkspace(w) }); err != nil {
		ts.T.Fatal(err)
	}
}

// DeleteTask sets DeletedAt of the task id.
func (ts *Session) DeleteTask(id string) {
	ts.T.Helper()
	s, ix, err := ts.Open()
	if err != nil {
		ts.T.Fatal(err)
	}
	e, err := ts.Resolve(ix, id)
	if err != nil {
		ts.T.Fatal(err)
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
		ts.T.Fatal(err)
	}
}

// UseAccount makes github.com/alice the account of config.yaml.
func (ts *Session) UseAccount() {
	ts.T.Helper()
	ts.AppendConfig("accounts:\n  - name: alice\n    host: github.com\n    login: alice\n")
	ts.Renew()
}

// WantErr checks that err has the kind and the code.
func WantErr(t *testing.T, err error, kind usecase.Kind, code string) {
	t.Helper()
	e := usecase.AsError(err)
	if err == nil {
		t.Fatalf("no error; want %v %q", kind, code)
	}
	if usecase.KindOf(err) != kind {
		t.Errorf("KindOf(%v) = %v; want %v", err, usecase.KindOf(err), kind)
	}
	if code != "" && e.Code != code {
		t.Errorf("Code of %v = %+v; want %q", err, e, code)
	}
}

func AsError(err error, target **usecase.Error) bool { return errors.As(err, target) }

const BobAccount = "\naccounts:\n  - name: bob\n    host: github.com\n    login: bob\n"

// AsWriter makes the accounts of config.yaml those of from's replacement to,
// and returns a session of its own, which has not read the writer yet.
func (ts *Session) AsWriter(from, to string) *usecase.Session {
	ts.T.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.T.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigFile, []byte(strings.Replace(string(b), from, to, 1)), 0o600); err != nil {
		ts.T.Fatal(err)
	}
	return &usecase.Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn}
}

// MoveFx is org-a (API) with a project API-2, its child API-3 and that
// one's child API-4, and API-5 outside; API-3 has a comment and a worklog
// entry; API-4 is blocked by API-1 and API-5 by API-3. org-b is WEB.
type MoveFx struct {
	TS                     *Session
	P, C, G, O, First      model.ULID
	Comment, Worklog       model.ULID
	CommentRel, WorklogRel string
}

func NewMoveFx(t *testing.T) *MoveFx {
	t.Helper()
	ts := NewTwoStore(t)
	a := ts.ForStore("org-a")
	add := func(title, parent, kind string) model.ULID {
		r, err := tasks.Add(a, tasks.AddInput{Title: title, Parent: parent, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		return r.ULID
	}
	f := &MoveFx{TS: ts}
	f.P = add("P", "", "project")
	f.C = add("C", "API-2", "")
	f.G = add("G", "API-3", "")
	f.O = add("O", "", "")
	ix := MustIndex(t, ts, "org-a")
	e, err := a.Resolve(ix, "API-1")
	if err != nil {
		t.Fatal(err)
	}
	f.First = e.ULID
	if _, err := tasks.Dep(a, "API-4", "API-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Dep(a, "API-5", "API-3", true); err != nil {
		t.Fatal(err)
	}
	f.Comment, f.Worklog = model.NewULID(), model.NewULID()
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Update(s, store.Op{Name: "comment", Task: f.C}, func(tx *store.Tx) error {
		if err := tx.PutComment(f.C, &model.Comment{ULID: f.Comment, Author: "github.com/alice", Body: "first"}); err != nil {
			return err
		}
		return tx.PutWorklog(f.C, &model.Worklog{ULID: f.Worklog, Author: "github.com/alice", Duration: 3600, Body: "work"})
	})
	if err != nil {
		t.Fatal(err)
	}
	f.CommentRel, f.WorklogRel = model.CommentPath(f.C, f.Comment), model.WorklogPath(f.C, f.Worklog)
	return f
}

func (f *MoveFx) In() stores.MoveStoreInput {
	return stores.MoveStoreInput{Arg: "API-2", To: "org-b", Yes: true}
}

func (f *MoveFx) Head(t *testing.T, name string) string {
	t.Helper()
	s, err := f.TS.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

func (f *MoveFx) Log(t *testing.T, name string) []gitrepo.LogEntry {
	t.Helper()
	s, err := f.TS.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	l, err := LogOf(s.Repo(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// SetSchema changes the schema of org-a.
func (f *MoveFx) SetSchema(t *testing.T, fn func(*schema.Schema)) {
	t.Helper()
	ss := f.TS.ForStore("org-a")
	s, err := ss.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	sc, err := stores.CloneSchemaOf(s.Schema())
	if err != nil {
		t.Fatal(err)
	}
	fn(sc)
	if _, err := ss.Update(s, store.Op{Name: "schema"}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
}

func MustCode(t *testing.T, err error, kind usecase.Kind, code string) {
	t.Helper()
	if e := usecase.AsError(err); err == nil || e.Kind != kind || e.Code != code {
		t.Fatalf("err = %v; want kind %v code %s", err, kind, code)
	}
}

const AliceAccount = "\naccounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

// MovedFixture moves org-a's API-1 to org-b: org-b gets the task as WEB-2
// (API-1 as a former id), and org-a's file becomes a forward.
// It returns the ULID of the task.
func MovedFixture(t *testing.T, ts *Session) model.ULID {
	t.Helper()
	sa, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	sb, ix, err := ts.ForStore("org-b").Open()
	if err != nil {
		t.Fatal(err)
	}
	ea, err := ts.ForStore("org-a").Resolve(MustIndex(t, ts, "org-a"), "API-1")
	if err != nil {
		t.Fatal(err)
	}
	ulid := ea.ULID
	nt := &model.Task{ULID: ulid, Title: "moved", Kind: "task", Status: "todo", Workspace: ix.Workspaces()[0].ULID, FormerIDs: []string{"API-1"}}
	if _, err := ts.ForStore("org-b").UpdateCommit(sb, store.Op{Name: "move-in", Task: ulid}, func(tx *store.Tx) error { return tx.PutTask(nt) }); err != nil {
		t.Fatal(err)
	}
	fwd := "---\nulid: " + ulid.String() + "\nmoved_to: " + sb.Meta().ULID.String() + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n"
	if _, err := sa.Update(store.Op{Name: "move-out", Author: MovedAuthor, Detail: "API-1 org-a task"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(ulid), []byte(fwd))
	}); err != nil {
		t.Fatal(err)
	}
	return ulid
}

func MustIndex(t *testing.T, ts *Session, name string) *index.Index {
	t.Helper()
	_, ix, err := ts.ForStore(name).Open()
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func HeadOf(t *testing.T, s *store.Store) string {
	t.Helper()
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

// UlidOf is the ULID of the task arg names.
func UlidOf(t *testing.T, ts *Session, arg string) model.ULID {
	t.Helper()
	r, err := browse.Show(ts.Session, arg)
	if err != nil {
		t.Fatal(err)
	}
	u, err := model.ParseULID(r.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func WantCode(t *testing.T, what string, err error, kind usecase.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: no error; want %v %s", what, kind, code)
		return
	}
	if e := usecase.AsError(err); e.Kind != kind || e.Code != code {
		t.Errorf("%s: %v is %v %q; want %v %q", what, err, e.Kind, e.Code, kind, code)
	}
}

func MakeStoreReadOnly(t *testing.T, ts *Session, name string) {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := replicate.WriteAccess(s, replicate.Access{ReadOnly: true, Reason: "test", Checked: ts.Now()}); err != nil {
		t.Fatal(err)
	}
}

func OpenSet(t *testing.T, es ...config.StoreEntry) *mount.Set {
	t.Helper()
	set := mount.Open(&config.Config{Stores: es}, mount.Options{})
	return set
}

// TestSchema is the default schema with a property of each other type.
func TestSchema() *schema.Schema {
	sc := schema.Default()
	sc.Props = append(sc.Props,
		schema.Prop{Name: "flag", Type: schema.TypeBool},
		schema.Prop{Name: "at", Type: schema.TypeDatetime},
		schema.Prop{Name: "note", Type: schema.TypeText},
		schema.Prop{Name: "tags", Type: schema.TypeMultiEnum, Values: []string{"a", "b"}},
	)
	return sc
}

func MustAssigns(t *testing.T, args ...string) []usecase.Assign {
	t.Helper()
	as, err := usecase.ParseAssigns(args)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func MustULID(t *testing.T, s string) model.ULID {
	t.Helper()
	u, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

const UseGoneWarning = "the project of hedgehug use is not found (hedgehug use --clear removes it)"

// AddProjectIn adds a project to the store name and returns its id.
func AddProjectIn(t *testing.T, ts *Session, name, title string) string {
	t.Helper()
	r, err := tasks.Add(ts.ForStore(name), tasks.AddInput{Title: title, Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.View()
	if err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func LoadContext(t *testing.T, ts *Session) current.State {
	t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	return current.Load(p.DataDir, func(m string) { t.Errorf("context.json warned: %s", m) })
}

// FakeSession is a session whose github.com is a fakegh; accounts is added to
// config.yaml. HH_TOKEN is the token of alice.
func FakeSession(t *testing.T, st fakegh.State, accounts string) (*Session, *fakegh.Server) {
	t.Helper()
	zero := 0
	st.DeviceInterval = &zero
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	fake := fakegh.New(st)
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	ts := New(t)
	ts.Env["HH_TOKEN"] = "t-alice"
	if accounts == "" {
		ts.DropDefaultAccount()
	}
	ts.AppendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n" + accounts)
	return ts, fake
}

func ConfigText(t *testing.T, ts *Session) string {
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

func NotExist(t *testing.T, path, what string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s is left: %v", what, err)
	}
}

func StoreDirOf(t *testing.T, ts *Session, name string) string {
	t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(p.StoresDir, name)
}

// FreezeConfig sets the modification time of config.yaml to the past and
// returns the check that the file was not written since: its time and its
// bytes are what they were. what says what must not have written it.
func (ts *Session) FreezeConfig(what string) (unchanged func()) {
	ts.T.Helper()
	p := filepath.Join(ts.Env["HEDGEHUG_HOME"], "config.yaml")
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p, old, old); err != nil {
		ts.T.Fatal(err)
	}
	before, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.T.Fatal(err)
	}
	return func() {
		ts.T.Helper()
		if fi, err := os.Stat(p); err != nil {
			ts.T.Error(err)
		} else if !fi.ModTime().Equal(old) {
			ts.T.Errorf("config.yaml was written by %s: modified %v", what, fi.ModTime())
		}
		if after, _ := os.ReadFile(p); string(after) != string(before) { //nolint:gosec // the config of the test
			ts.T.Errorf("config.yaml changed by %s:\n%s", what, after)
		}
	}
}

// HookConfigWrite makes ts write config.yaml through another Config, right
// before the operation updates it, as another process could.
func HookConfigWrite(t *testing.T, ts *Session, fn func(*config.Config) error) {
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

// LogOf is the first-parent log of HEAD of r, newest first, capped at limit
// when it is above zero.
func LogOf(r *gitrepo.Repo, limit int) ([]gitrepo.LogEntry, error) {
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	return r.LogFrom(head, limit)
}

// CfgStore is a store that SetConfig writes into config.yaml; account is
// empty for a store with no account: of its own.
type CfgStore struct{ Name, Account string }

// Keyring is a keychain in memory. onSet and onDelete run before the call, and
// failSet makes Set fail.
type Keyring struct {
	Mu         sync.Mutex
	M          map[string]string
	OnSet      func()
	OnDelete   func()
	FailSet    error
	FailDelete error
}

func NewKeyring() *Keyring { return &Keyring{M: map[string]string{}} }

func (k *Keyring) Get(_, user string) (string, error) {
	k.Mu.Lock()
	defer k.Mu.Unlock()
	v, ok := k.M[user]
	if !ok {
		return "", auth.ErrNotFound
	}
	return v, nil
}

func (k *Keyring) Set(_, user, secret string) error {
	if k.OnSet != nil {
		k.OnSet()
	}
	k.Mu.Lock()
	defer k.Mu.Unlock()
	if k.FailSet != nil {
		return k.FailSet
	}
	k.M[user] = secret
	return nil
}

func (k *Keyring) Delete(_, user string) error {
	if k.OnDelete != nil {
		k.OnDelete()
	}
	k.Mu.Lock()
	defer k.Mu.Unlock()
	if k.FailDelete != nil {
		return k.FailDelete
	}
	if _, ok := k.M[user]; !ok {
		return auth.ErrNotFound
	}
	delete(k.M, user)
	return nil
}

func (k *Keyring) Has(user string) (string, bool) {
	k.Mu.Lock()
	defer k.Mu.Unlock()
	v, ok := k.M[user]
	return v, ok
}

var MovedAuthor = gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}

// MakeReadOnly writes access.json of the test store as a read-only store.
func MakeReadOnly(t *testing.T, ts *Session, reason string) {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := replicate.WriteAccess(s, replicate.Access{ReadOnly: true, Reason: reason, Checked: ts.Now()}); err != nil {
		t.Fatal(err)
	}
}

// BrokenForward puts a forward file with a key it must not have in org-a.
func (f *MoveFx) BrokenForward(t *testing.T) model.ULID {
	t.Helper()
	u := model.NewULID()
	a := f.TS.ForStore("org-a")
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

// OldForward puts a forward file that moved at in org-a.
func (f *MoveFx) OldForward(t *testing.T, at time.Time) model.ULID {
	t.Helper()
	u := model.NewULID()
	a := f.TS.ForStore("org-a")
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

// AddSprint puts the sprint "Sprint 1" on C (API-3) of org-a and returns its
// ULID, as TestMoveStoreUnlinksSprintAndReleases does.
func (f *MoveFx) AddSprint(t *testing.T) model.ULID {
	t.Helper()
	a := f.TS.ForStore("org-a")
	as, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	var sprint model.ULID
	_, err = a.Update(as, store.Op{Name: "sprint"}, func(tx *store.Tx) error {
		c, err := tx.Task(f.C)
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

// AddGoneStore sets up a store called gone in config.yaml at a path where
// there is no store, with a remote when withRemote.
func (ts *Session) AddGoneStore(t *testing.T, withRemote bool) {
	t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	var remotes []string
	if withRemote {
		remotes = []string{filepath.Join(t.TempDir(), "remote.git")}
	}
	if err := c.AddStore("gone", filepath.Join(t.TempDir(), "gone"), remotes); err != nil {
		t.Fatal(err)
	}
	if err := c.SetStoreAccount("gone", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.DropMount()
	ts.Warns = nil
}

// CountContaining is how many of warns have sub in them.
func CountContaining(warns []string, sub string) int {
	var n int
	for _, w := range warns {
		if strings.Contains(w, sub) {
			n++
		}
	}
	return n
}
