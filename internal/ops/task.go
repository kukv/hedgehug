package ops

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// TaskResult is what a write to one task returns: the index read before the
// write (the CLI prints from it after refreshing), the task and whether a
// commit was made.
type TaskResult struct {
	Index     *index.Index
	ULID      model.ULID
	Committed bool
	Commit    string // hex hash of the commit made; "" when nothing was committed
}

// View is the task as the write left it. It refreshes the index, so a caller
// that answers a request from it never returns what the task was before.
func (r TaskResult) View() (*TaskView, error) {
	if err := r.Index.Refresh(); err != nil {
		return nil, err
	}
	e, ok := r.Index.Task(r.ULID)
	if !ok {
		return nil, newErr(KindNotFound, "task.gone", fmt.Errorf("%s is no longer in the store", r.ULID), "id", r.ULID.String())
	}
	return TaskViewOf(r.Index, e), nil
}

// AddInput is what Add makes a task from. Empty Kind and Status take the
// schema's defaults; Parent and Workspace are names, as the user gives them.
type AddInput struct {
	Title, Parent, Workspace, Kind, Status string
	Assigns                                []Assign
}

// Add adds a task.
func (ss *Session) Add(in AddInput) (TaskResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TaskResult{}, err
	}
	sc := ix.Schema()
	t := &model.Task{ULID: model.NewULID(), Title: in.Title, Kind: in.Kind, Status: in.Status}
	if t.Kind == "" {
		k, ok := sc.DefaultKind()
		if !ok {
			return TaskResult{}, newErr(KindInvalid, "schema.no_default", errors.New("the schema has no default kind; give one with --kind"), "what", "kind")
		}
		t.Kind = k.Name
	}
	if t.Status == "" {
		st, ok := sc.DefaultStatus(schema.CategoryTodo)
		if !ok {
			return TaskResult{}, newErr(KindInvalid, "schema.no_default", errors.New("the schema has no default todo status; give one with --status"), "what", "status")
		}
		t.Status = st.Name
	}
	if in.Parent != "" {
		p, err := ss.ResolveRef(ix, in.Parent)
		if err != nil {
			return TaskResult{}, err
		}
		if err := LiveParent(ix, p); err != nil {
			return TaskResult{}, err
		}
		t.Parent = &p.ULID
		t.Workspace = p.Workspace
	}
	if in.Workspace != "" {
		w, err := ResolveWorkspace(ix, in.Workspace)
		if err != nil {
			return TaskResult{}, err
		}
		if in.Parent != "" && w.ULID != t.Workspace {
			pw, ok := ix.Workspace(t.Workspace)
			if !ok {
				return TaskResult{}, newErr(KindInvalid, "task.workspace_mismatch", fmt.Errorf("the parent's workspace %s is not in this store", t.Workspace))
			}
			return TaskResult{}, newErr(KindInvalid, "task.workspace_mismatch", fmt.Errorf("the parent is in workspace %s", pw.Name))
		}
		t.Workspace = w.ULID
	} else if in.Parent == "" {
		wss := ix.Workspaces()
		if len(wss) == 0 {
			return TaskResult{}, newErr(KindConflict, "workspace.none", errors.New("the store has no workspace; add one with hedgehug workspace add <name> --prefix <PREFIX>"))
		}
		if len(wss) != 1 {
			names := make([]string, len(wss))
			for i, w := range wss {
				names[i] = fmt.Sprintf("%s (%s)", w.Name, w.IDPrefix)
			}
			// KindInvalid, not KindUsage: the CLI exits 1 here.
			return TaskResult{}, newErr(KindInvalid, "workspace.choose", errors.New("there are several workspaces; give one with --workspace: "+strings.Join(names, ", ")), "names", names)
		}
		t.Workspace = wss[0].ULID
	}
	for _, as := range in.Assigns {
		if err := ApplyAssign(t, as, sc, ss.ResolveULID(ix)); err != nil {
			return TaskResult{}, err
		}
	}
	if t.Rank, err = EndRank(ix, t.Parent, t.Workspace, t.ULID); err != nil {
		return TaskResult{}, err
	}
	if t.CreatedBy, err = ss.AccountID(); err != nil {
		return TaskResult{}, err
	}
	if t.CreatedBy != "" {
		AddWatchers(t, t.CreatedBy)
	}
	watchNewAssignees(t, nil)
	hash, err := ss.UpdateCommit(s, store.Op{Name: "add", Task: t.ULID}, func(tx *store.Tx) error { return tx.PutTask(t) })
	if err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: ix, ULID: t.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// Set applies assigns to the task arg names.
func (ss *Session) Set(arg string, assigns []Assign) (TaskResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return TaskResult{}, err
	}
	if ts != ss {
		return ts.Set(arg, assigns)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return TaskResult{}, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return TaskResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "set", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		assignees := slices.Clone(t.Assignees)
		for _, as := range assigns {
			if err := ApplyAssign(t, as, ix.Schema(), ss.ResolveULID(ix)); err != nil {
				return err
			}
		}
		watchNewAssignees(t, assignees)
		return tx.PutTask(t)
	})
	if err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// SetStatus sets the status of the task arg names; an empty status means the
// default status of the done category.
func (ss *Session) SetStatus(arg, name string) (TaskResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return TaskResult{}, err
	}
	if ts != ss {
		return ts.SetStatus(arg, name)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TaskResult{}, err
	}
	sc := ix.Schema()
	if name == "" {
		st, ok := sc.DefaultStatus(schema.CategoryDone)
		if !ok {
			return TaskResult{}, newErr(KindInvalid, "schema.no_default", errors.New("the schema has no default done status"), "what", "done_status")
		}
		name = st.Name
	} else if _, ok := sc.Status(name); !ok {
		names := make([]string, len(sc.Statuses))
		for i, st := range sc.Statuses {
			names[i] = st.Name
		}
		return TaskResult{}, newErr(KindInvalid, "status.unknown", fmt.Errorf("%q is not a status (statuses: %s)", name, strings.Join(names, ", ")), "name", name)
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return TaskResult{}, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return TaskResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "status", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		t.Status = name
		return tx.PutTask(t)
	})
	if err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// Dep makes the task arg names blocked by the task blockerArg names, or, with
// add false, no longer blocked by it.
func (ss *Session) Dep(arg, blockerArg string, add bool) (TaskResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return TaskResult{}, err
	}
	if ts != ss {
		return ts.Dep(arg, blockerArg, add)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return TaskResult{}, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return TaskResult{}, err
	}
	b, err := ss.ResolveRef(ix, blockerArg)
	if err != nil {
		return TaskResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "dep", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		has := slices.Contains(t.BlockedBy, b.ULID)
		switch {
		case add && has:
			return newErr(KindInvalid, "dep.invalid", fmt.Errorf("%s is already blocked by %s", RefName(ix, e.ULID), RefName(ix, b.ULID)))
		case add:
			t.BlockedBy = append(t.BlockedBy, b.ULID)
		case !has:
			return newErr(KindInvalid, "dep.invalid", fmt.Errorf("%s is not blocked by %s", RefName(ix, e.ULID), RefName(ix, b.ULID)))
		default:
			t.BlockedBy = slices.DeleteFunc(t.BlockedBy, func(x model.ULID) bool { return x == b.ULID })
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

var errNoAccounts = newErr(KindConflict, "account.none", errors.New("accounts are not set up in config.yaml; add one to accounts"))

// Watch adds the writer of the store to the watchers of the task arg
// names, or, with on false, takes it out.
func (ss *Session) Watch(arg string, on bool) (TaskResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return TaskResult{}, err
	}
	if ts != ss {
		return ts.Watch(arg, on)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TaskResult{}, err
	}
	acct, ok, err := ss.Writer()
	if err != nil {
		return TaskResult{}, err
	}
	if !ok {
		return TaskResult{}, errNoAccounts
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return TaskResult{}, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return TaskResult{}, err
	}
	name := "unwatch"
	if on {
		name = "watch"
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: name, Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		if on {
			AddWatchers(t, acct.ID())
		} else {
			t.Watchers = slices.DeleteFunc(t.Watchers, func(w string) bool { return config.SameAccount(w, acct.ID()) })
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// AddWatchers adds the accounts that t does not have yet to its watchers,
// at the end. Accounts compare without case.
func AddWatchers(t *model.Task, accts ...string) {
	for _, acct := range accts {
		if !slices.ContainsFunc(t.Watchers, func(w string) bool { return config.SameAccount(w, acct) }) {
			t.Watchers = append(t.Watchers, acct)
		}
	}
}

// mentionedAccounts returns the accounts body mentions as account IDs: a
// @login gets the host of the writer, a @host/login is as written. Without
// a writer host a @login names no account and is left out.
func mentionedAccounts(body string, writerHost string) []string {
	var out []string
	for _, m := range index.Mentions(body) {
		switch {
		case strings.Contains(m, "/"):
			out = append(out, m)
		case writerHost != "":
			out = append(out, writerHost+"/"+m)
		}
	}
	return out
}

// NewMentions returns the accounts now mentions and old did not.
func NewMentions(old, now, writerHost string) []string {
	was := mentionedAccounts(old, writerHost)
	var out []string
	for _, m := range mentionedAccounts(now, writerHost) {
		if !slices.ContainsFunc(was, func(w string) bool { return config.SameAccount(w, m) }) {
			out = append(out, m)
		}
	}
	return out
}

// WriterHost is the host of the account that writes to the store, "" when
// the store has none.
func (ss *Session) WriterHost() (string, error) {
	acct, ok, err := ss.Writer()
	if err != nil || !ok {
		return "", err
	}
	return acct.Host, nil
}

// watchNewAssignees adds to the watchers of t the assignees it has now and
// did not in before.
func watchNewAssignees(t *model.Task, before []string) {
	for _, as := range t.Assignees {
		if !slices.ContainsFunc(before, func(b string) bool { return config.SameAccount(b, as) }) {
			AddWatchers(t, as)
		}
	}
}

// WatchInTx adds accts to the watchers of the task in the operation; the
// task is not written when they all watch it already.
func WatchInTx(tx *store.Tx, task model.ULID, accts ...string) error {
	if len(accts) == 0 {
		return nil
	}
	t, err := tx.Task(task)
	if err != nil {
		return err
	}
	n := len(t.Watchers)
	AddWatchers(t, accts...)
	if len(t.Watchers) == n {
		return nil
	}
	return tx.PutTask(t)
}

// NotDeleted refuses to change the task e when it is deleted, and points
// to restore.
func NotDeleted(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt != nil {
		return newErr(KindConflict, "task.deleted", fmt.Errorf("%s is deleted; restore %s first", RefName(ix, e.ULID), idOrULIDShown(ix, e)), "id", TaskFileName(e))
	}
	return nil
}

// LiveParent refuses a deleted task as a parent: live tasks under it would
// block its purge and the restore of its other children.
func LiveParent(ix *index.Index, p *index.Entry) error {
	if err := NotDeleted(ix, p); err != nil {
		return newErr(KindConflict, "task.parent_deleted", fmt.Errorf("the parent %w", err), "id", TaskFileName(p))
	}
	return nil
}

// ResolveWorkspace finds the workspace arg names.
func ResolveWorkspace(ix *index.Index, arg string) (*model.Workspace, error) {
	w, err := ix.ResolveWorkspace(arg)
	var ae *index.AmbiguousWorkspaceError
	if errors.As(err, &ae) {
		lines := make([]string, len(ae.Candidates))
		for i, c := range ae.Candidates {
			lines[i] = fmt.Sprintf("%s (%s, %s)", c.Name, c.IDPrefix, c.ULID)
		}
		return nil, newErr(KindAmbiguous, "workspace.ambiguous", &ambiguousError{input: arg, noun: "workspaces", lines: lines}, "input", arg)
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, newErr(KindNotFound, "workspace.not_found", err, "input", arg)
	}
	return w, err
}

// Siblings returns the tasks under parent (the root when nil, which also
// holds the tasks whose parent is missing, as Roots does) in workspace ws
// that are not deleted, leaving out skip, by rank.
func Siblings(ix *index.Index, parent *model.ULID, ws, skip model.ULID) []*index.Entry {
	var all []*index.Entry
	if parent == nil {
		all = ix.Roots()
	} else {
		all = ix.Children(*parent)
	}
	out := all[:0:0]
	for _, e := range all {
		if e.DeletedAt == nil && e.ULID != skip && e.Workspace == ws {
			out = append(out, e)
		}
	}
	return out
}

// EndRank returns a rank after every sibling under parent.
func EndRank(ix *index.Index, parent *model.ULID, ws, skip model.ULID) (model.Rank, error) {
	sib := Siblings(ix, parent, ws, skip)
	var last model.Rank
	if len(sib) > 0 {
		last = sib[len(sib)-1].Rank
	}
	return model.Between(last, "")
}
