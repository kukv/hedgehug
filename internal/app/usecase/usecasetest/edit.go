package usecasetest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/auth"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// Empty is a Session over a fresh HOME and HEDGEHUG_HOME whose config.yaml
// has the default account only. It makes no store.
func Empty(t *testing.T, now func() time.Time) *Session {
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
	ts.Session = &usecase.Session{
		Getenv: func(k string) string { return ts.Env[k] },
		Now:    now,
		Warn:   func(m string) { ts.Warns = append(ts.Warns, m) },
	}
	ts.AppendDefaultAccount()
	return ts
}

// Attach is a Session over the data directory getenv points at, as the
// command sees it. It makes neither a store nor a config: the caller has them
// (the test environment of the cli shares its directories with it).
func Attach(t *testing.T, getenv func(string) string, now func() time.Time) *Session {
	t.Helper()
	ts := &Session{T: t, Env: map[string]string{
		"HOME":          getenv("HOME"),
		"HEDGEHUG_HOME": getenv("HEDGEHUG_HOME"),
	}}
	ts.Session = &usecase.Session{
		Getenv: getenv,
		Now:    now,
		Warn:   func(m string) { ts.Warns = append(ts.Warns, m) },
	}
	return ts
}

// Author is who a commit of Edit is by.
type Author struct{ Name, Email string }

var (
	TAuthor     = Author{"T", "t@example.com"}
	AliceAuthor = Author{"Alice", "alice@example.com"}
	// AccountAuthor is who the commits of TestAccount are by.
	AccountAuthor = Author{config.CommitAuthor(TestAccount).Name, config.CommitAuthor(TestAccount).Email}
)

// Tx is the transaction of an Edit, written in the types of the domain.
type Tx struct {
	tx *store.Tx
	s  *store.Store
}

func (x *Tx) Task(u model.ULID) (*model.Task, error)           { return x.tx.Task(u) }
func (x *Tx) PutTask(t *model.Task) error                      { return x.tx.PutTask(t) }
func (x *Tx) PutWorkspace(w *model.Workspace) error            { return x.tx.PutWorkspace(w) }
func (x *Tx) PutSprint(s *model.Sprint) error                  { return x.tx.PutSprint(s) }
func (x *Tx) PutRelease(r *model.Release) error                { return x.tx.PutRelease(r) }
func (x *Tx) PutComment(t model.ULID, c *model.Comment) error  { return x.tx.PutComment(t, c) }
func (x *Tx) PutSchema(s *schema.Schema) error                 { return x.tx.PutSchema(s) }
func (x *Tx) PutFile(rel string, data []byte) error            { return x.tx.PutFile(rel, data) }
func (x *Tx) Workspace(u model.ULID) (*model.Workspace, error) { return x.tx.Workspace(u) }

// Comments are the comments of task as the store has them committed.
func (x *Tx) Comments(task model.ULID) ([]*model.Comment, error) {
	var out []*model.Comment
	for c, err := range x.s.Comments(task) {
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Workspaces are the workspaces of the store as it has them committed.
func (x *Tx) Workspaces() ([]*model.Workspace, error) {
	var out []*model.Workspace
	for w, err := range x.s.Workspaces() {
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// StoreDir is the directory of the store storeName.
func (ts *Session) StoreDir(storeName string) string {
	ts.T.Helper()
	return StoreDirOf(ts.T, ts, storeName)
}

// openAt opens the store storeName by its directory, as a test that writes to
// the store beside the command does.
func (ts *Session) openAt(storeName string) *store.Store {
	ts.T.Helper()
	s, err := store.Open(ts.StoreDir(storeName))
	if err != nil {
		ts.T.Fatal(err)
	}
	ts.T.Cleanup(func() { _ = s.Repo().Close() })
	return s
}

// Edit opens the store storeName and calls its Update with Op{Name: op,
// Author: by}. It is not Session.Update, which writes as the account of the
// session.
func (ts *Session) Edit(storeName, op string, by Author, fn func(*Tx) error) error {
	ts.T.Helper()
	return ts.EditTask(storeName, op, model.ULID{}, by, fn)
}

// EditTask is Edit for an operation of the task task, which the subject and
// the Hedgehug-Task trailer of the commit name.
func (ts *Session) EditTask(storeName, op string, task model.ULID, by Author, fn func(*Tx) error) error {
	ts.T.Helper()
	s := ts.openAt(storeName)
	_, err := s.Update(store.Op{Name: op, Task: task, Author: gitrepo.Signature{Name: by.Name, Email: by.Email}}, func(tx *store.Tx) error {
		return fn(&Tx{tx: tx, s: s})
	})
	return err
}

// MustEdit is Edit that fails the test on an error.
func (ts *Session) MustEdit(storeName, op string, by Author, fn func(*Tx) error) {
	ts.T.Helper()
	if err := ts.Edit(storeName, op, by, fn); err != nil {
		ts.T.Fatal(err)
	}
}

// TaskByULID reads the task u from the store storeName.
func (ts *Session) TaskByULID(storeName string, u model.ULID) *model.Task {
	ts.T.Helper()
	t, err := ts.openAt(storeName).Task(u)
	if err != nil {
		ts.T.Fatal(err)
	}
	return t
}

// TaskExists reports whether the store storeName has the task u.
func (ts *Session) TaskExists(storeName string, u model.ULID) bool {
	ts.T.Helper()
	_, err := ts.openAt(storeName).Task(u)
	return err == nil
}

// WorkspaceByULID reads the workspace u from the store storeName.
func (ts *Session) WorkspaceByULID(storeName string, u model.ULID) *model.Workspace {
	ts.T.Helper()
	w, err := ts.openAt(storeName).Workspace(u)
	if err != nil {
		ts.T.Fatal(err)
	}
	return w
}

// WorkspaceNamed is the ULID of the workspace name of the store storeName.
func (ts *Session) WorkspaceNamed(storeName, name string) model.ULID {
	ts.T.Helper()
	for w, err := range ts.openAt(storeName).Workspaces() {
		if err != nil {
			ts.T.Fatal(err)
		}
		if w.Name == name {
			return w.ULID
		}
	}
	ts.T.Fatalf("no workspace %s in %s", name, storeName)
	return model.ULID{}
}

// CommentsOf are the comments of the task, deleted ones included.
func (ts *Session) CommentsOf(storeName string, task model.ULID) []*model.Comment {
	ts.T.Helper()
	var out []*model.Comment
	for c, err := range ts.openAt(storeName).Comments(task) {
		if err != nil {
			ts.T.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

// StoreMeta is store.yaml of the store storeName.
func (ts *Session) StoreMeta(storeName string) model.StoreMeta {
	ts.T.Helper()
	return *ts.openAt(storeName).Meta()
}

// SchemaOf is the schema of the store storeName.
func (ts *Session) SchemaOf(storeName string) *schema.Schema {
	ts.T.Helper()
	return ts.openAt(storeName).Schema()
}

// TaskULIDs are the ULIDs of the tasks of the store storeName.
func (ts *Session) TaskULIDs(storeName string) []model.ULID {
	ts.T.Helper()
	var out []model.ULID
	for e, err := range ts.openAt(storeName).Tasks() {
		if err != nil {
			ts.T.Fatal(err)
		}
		out = append(out, e.ULID)
	}
	return out
}

// WriteCycle makes x the parent of y and y the parent of x, by writing the
// files and committing them with go-git, past the store's checks.
func (ts *Session) WriteCycle(storeName string, x, y model.ULID) {
	ts.T.Helper()
	t := ts.T
	s := ts.openAt(storeName)
	dir := ts.StoreDir(storeName)
	for _, pair := range [][2]model.ULID{{x, y}, {y, x}} {
		tk, err := s.Task(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		parent := pair[1]
		tk.Parent = &parent
		b, err := tk.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(model.TaskPath(pair[0]))), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	// Add only the two files: .hedgehug/local/ holds the index cache.
	for _, ulid := range []model.ULID{x, y} {
		if _, err := w.Add(model.TaskPath(ulid)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Commit("make a cycle", &git.CommitOptions{Author: &object.Signature{Name: "T", Email: "t@example.com", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}
}

// ResolveErr is the error of the index of the test store resolving arg.
func (ts *Session) ResolveErr(arg string) error {
	ts.T.Helper()
	s := ts.openAt("me")
	ix, err := index.Open(s)
	if err != nil {
		ts.T.Fatal(err)
	}
	_, err = (&usecase.Session{}).Resolve(ix, arg)
	return err
}

// LocksFree reports whether none of the local locks names is held.
func (ts *Session) LocksFree(storeName string, names ...string) bool {
	ts.T.Helper()
	s := ts.openAt(storeName)
	for _, l := range names {
		if s.WithLocalLock(l, 0, func() error { return nil }) != nil {
			return false
		}
	}
	return true
}

// NoLeftover is nil when a rebuild left nothing beside the store at dir.
func (ts *Session) NoLeftover(dir string) error {
	left, err := store.FindLeftover(dir)
	if err != nil {
		return err
	}
	if !left.None() {
		return &leftoverError{left}
	}
	return nil
}

type leftoverError struct{ left store.Leftover }

func (e *leftoverError) Error() string { return "left beside the store: " + e.left.New }

// HistoryAtHead fails the test unless the saved history index of the store
// storeName is there and already at the newest commit: opening it again
// changes nothing on disk.
func (ts *Session) HistoryAtHead(storeName string) {
	ts.T.Helper()
	t := ts.T
	s := ts.openAt(storeName)
	before, err := os.ReadFile(s.LocalPath("history"))
	if err != nil {
		t.Fatalf("the history index of %s was not built: %v", storeName, err)
	}
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(s.LocalPath("history"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the history index of %s was not at HEAD", storeName)
	}
}

// Access is whether this machine may write to the store, as access.json has it.
type Access struct {
	ReadOnly bool
	Reason   string
	Checked  time.Time
}

const (
	ReasonGitHub      = replicate.ReasonGitHub
	ReasonNotWritable = replicate.ReasonNotWritable
)

// WriteAccess writes access.json of the store storeName.
func (ts *Session) WriteAccess(storeName string, a Access) {
	ts.T.Helper()
	if err := replicate.WriteAccess(ts.openAt(storeName), replicate.Access(a)); err != nil {
		ts.T.Fatal(err)
	}
}

// ReadAccess reads access.json of the store storeName.
func (ts *Session) ReadAccess(storeName string) Access {
	ts.T.Helper()
	a, err := replicate.ReadAccess(ts.openAt(storeName))
	if err != nil {
		ts.T.Fatal(err)
	}
	return Access(a)
}

// Member is a collaborator of members.json.
type Member struct{ ID, Permission string }

// WriteMembers writes members.json of the store storeName with the
// collaborators ms; a nil ms is collaborators that could not be read.
func (ts *Session) WriteMembers(storeName string, ms []Member) {
	ts.T.Helper()
	var cs []replicate.Collaborator
	if ms != nil {
		cs = make([]replicate.Collaborator, 0, len(ms))
		for _, m := range ms {
			cs = append(cs, replicate.Collaborator{ID: m.ID, Permission: m.Permission})
		}
	}
	if err := replicate.WriteMembers(ts.openAt(storeName), replicate.Members{Collaborators: cs}); err != nil {
		ts.T.Fatal(err)
	}
}

// ReadMembers reads the collaborators of members.json of the store storeName;
// known is false when they could not be read.
func (ts *Session) ReadMembers(storeName string) (ms []Member, known bool) {
	ts.T.Helper()
	m, err := replicate.ReadMembers(ts.openAt(storeName))
	if err != nil {
		ts.T.Fatal(err)
	}
	for _, c := range m.Collaborators {
		ms = append(ms, Member{ID: c.ID, Permission: c.Permission})
	}
	return ms, m.Collaborators != nil
}

// AgeChecks makes the last check of the access and of the members of the
// store storeName checked, whatever else they hold.
func (ts *Session) AgeChecks(storeName string, checked time.Time) {
	ts.T.Helper()
	s := ts.openAt(storeName)
	a, err := replicate.ReadAccess(s)
	if err != nil {
		ts.T.Fatal(err)
	}
	a.Checked = checked
	if err := replicate.WriteAccess(s, a); err != nil {
		ts.T.Fatal(err)
	}
	m, err := replicate.ReadMembers(s)
	if err != nil {
		ts.T.Fatal(err)
	}
	m.Checked = checked
	if err := replicate.WriteMembers(s, m); err != nil {
		ts.T.Fatal(err)
	}
}

// Unpushed is the number of commits of the store storeName that origin lacks.
func (ts *Session) Unpushed(storeName string) int {
	ts.T.Helper()
	n, err := replicate.Unpushed(ts.openAt(storeName).Repo())
	if err != nil {
		ts.T.Fatal(err)
	}
	return n
}

// InboxItem is a notice to put into the inbox of a store.
type InboxItem struct {
	Kind   string // the kind as inbox.jsonl has it: "renumbered", "repaired"...
	Task   model.ULID
	ID     string
	Path   string
	Commit string
	Time   time.Time
	Detail json.RawMessage
}

func (it InboxItem) item() inbox.Item {
	return inbox.Item{Kind: inbox.Kind(it.Kind), Task: it.Task, ID: it.ID, Path: it.Path, Commit: it.Commit, Time: it.Time, Detail: it.Detail}
}

// AddInboxItem puts it into the inbox of the store storeName and returns the
// ULID it got.
func (ts *Session) AddInboxItem(storeName string, it InboxItem) model.ULID {
	ts.T.Helper()
	added, err := inbox.Open(ts.openAt(storeName)).Add([]inbox.Item{it.item()})
	if err != nil || len(added) != 1 {
		ts.T.Fatalf("Add item = %v, %v", added, err)
	}
	return added[0].ULID
}

// InboxContent is what notifications say of it, as the index of the store
// storeName (or none) shows its tasks.
func InboxContent(ts *Session, storeName string, it InboxItem, withIndex bool) string {
	ts.T.Helper()
	var ix *index.Index
	if withIndex {
		var err error
		if ix, err = index.Open(ts.openAt(storeName)); err != nil {
			ts.T.Fatal(err)
		}
	}
	return notifications.InboxContent(ix, it.item(), storeName, false)
}

// NewOldStoreWithAttributes makes a store whose .gitattributes lacks a line of
// the merge driver, as an older version made it, and pushes it to remote.
func NewOldStoreWithAttributes(t *testing.T, remote string) {
	t.Helper()
	s, err := store.Create(filepath.Join(t.TempDir(), "old"), &model.StoreMeta{ULID: model.NewULID(), Name: "old"}, nil,
		&model.Workspace{ULID: model.NewULID(), Name: "old", IDPrefix: "OLD"}, gitrepo.Signature{Name: "A", Email: "a@example.com"}, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(store.Op{Name: "edit", Author: gitrepo.Signature{Name: "A", Email: "a@example.com"}}, func(tx *store.Tx) error {
		return tx.PutFile(store.AttributesPath, []byte("*.yaml merge=hedgehug\n"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().Push("origin"); err != nil {
		t.Fatal(err)
	}
}

// AccountIDs are the IDs (host/login) of the accounts of config.yaml, in order.
func (ts *Session) AccountIDs() []string {
	ts.T.Helper()
	var ids []string
	for _, a := range ts.Accounts() {
		ids = append(ids, a.ID())
	}
	return ids
}

// AccountTokenKind is the kind of the token and the ID of the account name in
// config.yaml; ok is false when there is none.
func (ts *Session) AccountTokenKind(name string) (kind, id string, ok bool) {
	ts.T.Helper()
	c, _, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	a, ok := c.Account(name)
	if !ok {
		return "", "", false
	}
	return a.Token.Kind, a.ID(), true
}

// DeviceTokens are the access and the refresh token of the account name, as
// the keychain k holds them.
func (ts *Session) DeviceTokens(k *Keyring, name string) (access, refresh string, err error) {
	ts.T.Helper()
	c, _, err := ts.Config()
	if err != nil {
		return "", "", err
	}
	a, ok := c.Account(name)
	if !ok {
		return "", "", auth.ErrNotFound
	}
	dt, err := auth.LoadDeviceToken(a, k)
	return dt.AccessToken, dt.RefreshToken, err
}

// MarkPending notes in the store storeName that it has a commit to push, since at.
func (ts *Session) MarkPending(storeName string, at time.Time) {
	ts.T.Helper()
	if err := replicate.MarkPending(ts.openAt(storeName), at); err != nil {
		ts.T.Fatal(err)
	}
}

func (ts *Session) syncState(storeName string) replicate.State {
	ts.T.Helper()
	st, err := replicate.ReadState(ts.openAt(storeName))
	if err != nil {
		ts.T.Fatal(err)
	}
	return st
}

// LastSuccess is when the store storeName was last synced; zero before the first.
func (ts *Session) LastSuccess(storeName string) time.Time {
	return ts.syncState(storeName).LastSuccess
}

// LastAttempt is when a sync of the store storeName last started.
func (ts *Session) LastAttempt(storeName string) time.Time {
	return ts.syncState(storeName).LastAttempt
}

// PendingSince is when the store storeName got the first commit that waits to be pushed.
func (ts *Session) PendingSince(storeName string) time.Time {
	return ts.syncState(storeName).PendingSince
}

// GoneTaskResult is the result of a write to a task that the index of the
// test store does not have.
func (ts *Session) GoneTaskResult() usecase.TaskResult {
	ts.T.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		ts.T.Fatal(err)
	}
	return usecase.NewTaskResult(ix, model.NewULID(), plumbing.ZeroHash)
}

// SyncIntervals are sync.interval and sync.push_interval of config.yaml.
func (ts *Session) SyncIntervals() (interval, pushInterval time.Duration) {
	ts.T.Helper()
	c, _, err := ts.Config()
	if err != nil {
		ts.T.Fatal(err)
	}
	return c.Sync.Interval, c.Sync.PushInterval
}
