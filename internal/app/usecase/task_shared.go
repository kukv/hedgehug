package usecase

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// TaskResult is what a write to one task returns: the index read before the
// write (the answer is made from it after refreshing), the task and whether a
// commit was made.
type TaskResult struct {
	index     *index.Index
	ULID      model.ULID
	Committed bool
	Commit    string // hex hash of the commit made; "" when nothing was committed
	// What a template added: the tasks under the task, and what it took off.
	Children int
	Detached []Detached
}

// NewTaskResult is the result of a write to the task ulid, read through ix,
// that made the commit hash (the zero hash for no commit).
func NewTaskResult(ix *index.Index, ulid model.ULID, hash plumbing.Hash) TaskResult {
	return TaskResult{index: ix, ULID: ulid, Committed: !hash.IsZero(), Commit: CommitHex(hash)}
}

// View is the task as the write left it, as a row of a list. It refreshes the
// index, so a caller that answers a request from it never returns what the
// task was before.
func (r TaskResult) View() (*TaskView, error) {
	if err := r.index.Refresh(); err != nil {
		return nil, err
	}
	e, ok := r.index.Task(r.ULID)
	if !ok {
		return nil, NewErr(KindNotFound, "task.gone", fmt.Errorf("%s is no longer in the store", r.ULID), "id", r.ULID.String())
	}
	return ListViewOf(r.index, e), nil
}

var ErrNoAccounts = NewErr(KindConflict, "account.none", errors.New("no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)"))

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

// DefaultKind is the kind of a new task that names none.
func DefaultKind(sc *schema.Schema) (string, error) {
	k, ok := sc.DefaultKind()
	if !ok {
		return "", NewErr(KindInvalid, "schema.no_default", errors.New("the schema has no default kind; give one with --kind"), "what", "kind")
	}
	return k.Name, nil
}

// DefaultTodo is the status of a new task that names none.
func DefaultTodo(sc *schema.Schema) (string, error) {
	st, ok := sc.DefaultStatus(schema.CategoryTodo)
	if !ok {
		return "", NewErr(KindInvalid, "schema.no_default", errors.New("the schema has no default todo status; give one with --status"), "what", "status")
	}
	return st.Name, nil
}

// OnlyWorkspace is the workspace of a new task that names none: the one the
// store has.
func OnlyWorkspace(ix *index.Index) (model.ULID, error) {
	wss := ix.Workspaces()
	if len(wss) == 0 {
		return model.ULID{}, NewErr(KindConflict, "workspace.none", errors.New("the store has no workspace; add one with hedgehug workspace add <name> --prefix <PREFIX>"))
	}
	if len(wss) != 1 {
		names := make([]string, len(wss))
		for i, w := range wss {
			names[i] = fmt.Sprintf("%s (%s)", w.Name, w.IDPrefix)
		}
		// KindInvalid, not KindUsage: the CLI exits 1 here.
		return model.ULID{}, NewErr(KindInvalid, "workspace.choose", errors.New("there are several workspaces; give one with --workspace: "+strings.Join(names, ", ")), "names", names)
	}
	return wss[0].ULID, nil
}

// NotDeletedrefuses to change the task e when it is deleted, and points
// to restore.
func NotDeleted(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt != nil {
		return NewErr(KindConflict, "task.deleted", fmt.Errorf("%s is deleted; restore %s first", RefName(ix, e.ULID), idOrULIDShown(ix, e)), "id", TaskFileName(e))
	}
	return nil
}

// LiveParent refuses a deleted task as a parent: live tasks under it would
// block its purge and the restore of its other children.
func LiveParent(ix *index.Index, p *index.Entry) error {
	if err := NotDeleted(ix, p); err != nil {
		return NewErr(KindConflict, "task.parent_deleted", fmt.Errorf("the parent %w", err), "id", TaskFileName(p))
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
		return nil, NewErr(KindAmbiguous, "workspace.ambiguous", &ambiguousError{input: arg, noun: "workspaces", lines: lines}, "input", arg)
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, NewErr(KindNotFound, "workspace.not_found", err, "input", arg)
	}
	return w, err
}

// Descendants returns the tasks under ulid for which follow is true, going
// down only through those, depth first in rank order. A parent cycle stops
// the walk.
func Descendants(ix *index.Index, ulid model.ULID, follow func(*index.Entry) bool) []*index.Entry {
	seen := map[model.ULID]bool{ulid: true}
	var out []*index.Entry
	var walk func(model.ULID)
	walk = func(p model.ULID) {
		for _, c := range ix.Children(p) {
			if seen[c.ULID] || !follow(c) {
				continue
			}
			seen[c.ULID] = true
			out = append(out, c)
			walk(c.ULID)
		}
	}
	walk(ulid)
	return out
}

func Every(*index.Entry) bool { return true }

// CountTasks counts the tasks of a workspace: the ones not deleted, or all
// of them when withDeleted.
func CountTasks(ix *index.Index, ws model.ULID, withDeleted bool) int {
	n := 0
	for _, e := range ix.Tasks() {
		if e.Workspace == ws && (withDeleted || e.DeletedAt == nil) {
			n++
		}
	}
	return n
}

// WarnMentions warns of each account that after mentions and before did not,
// when the members of s are known and the account is none of them: it cannot
// read the store. It is called after a write. When the members cannot be
// read, nothing can be said.
func (ss *Session) WarnMentions(s *store.Store, before, after string) {
	var fresh []string
	was := index.Mentions(before)
	for _, m := range index.Mentions(after) {
		if !slices.ContainsFunc(was, func(w string) bool { return index.MentionNames(m, w) }) {
			fresh = append(fresh, m)
		}
	}
	if len(fresh) == 0 {
		return
	}
	m, err := replicate.ReadMembers(s)
	if err != nil || m.Collaborators == nil {
		return
	}
	name := ss.storeNameOf(s)
	var warned []string
	for _, men := range fresh {
		if slices.ContainsFunc(m.Collaborators, func(c replicate.Collaborator) bool { return mentionIsCollaborator(men, c.ID) }) {
			continue
		}
		login := men[strings.LastIndex(men, "/")+1:]
		if slices.ContainsFunc(warned, func(w string) bool { return strings.EqualFold(w, login) }) {
			continue
		}
		warned = append(warned, login)
		ss.AddWarning(fmt.Sprintf("%s cannot read %s", login, name))
	}
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

// mentionIsCollaborator reports whether the mention names the collaborator
// with account ID id: @host/login that account, @login any with that login.
func mentionIsCollaborator(mention, id string) bool {
	if strings.Contains(mention, "/") {
		return config.SameAccount(mention, id)
	}
	return strings.EqualFold(mention, id[strings.LastIndex(id, "/")+1:])
}
