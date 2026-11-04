// Package tasks is the operations on tasks: add, set, edit, move, remove,
// restore and undo.
package tasks

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// AddInput is what Add makes a task from. Empty Kind and Status take the
// schema's defaults; Parent and Workspace are names, as the user gives them,
// read in the store the task goes to (Parent goes through ResolveRef). Store
// is that store ("" selects as the session does; one other than the
// session's own --store is store.conflict); Body is the Markdown body of the
// task, as it is written; Repos are the repo IDs to put on the task. Before
// and After are task names, as the user gives them: the task goes just before
// or after that sibling, as in Move, instead of after every sibling. With a
// sibling and no Parent, the task goes under the sibling's parent and in its
// workspace; a Parent or Workspace that does not fit the sibling is
// add.position.
type AddInput struct {
	Store                                  string
	Title, Parent, Workspace, Kind, Status string
	Body                                   string
	Before, After                          string
	Repos                                  []string
	// FirstRepo is put at the front of the repos once the Assigns are applied,
	// unless the task has it already (AddHere with Repo).
	FirstRepo string
	Assigns   []usecase.Assign
}

// Add adds a task.
func Add(ss *usecase.Session, in AddInput) (usecase.TaskResult, error) {
	if in.Store != "" && in.Store != ss.StoreName {
		if ss.StoreName != "" {
			return usecase.TaskResult{}, usecase.NewErr(usecase.KindUsage, "store.conflict",
				fmt.Errorf("the task goes to %s, but --store is %s", in.Store, ss.StoreName), "store", ss.StoreName, "named", in.Store)
		}
		to := in.Store
		in.Store = ""
		return Add(ss.ForStore(to), in)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	sc := ix.Schema()
	t := &model.Task{ULID: model.NewULID(), Title: in.Title, Kind: in.Kind, Status: in.Status, Body: in.Body, Repos: slices.Clone(in.Repos)}
	if t.Kind == "" {
		k, ok := sc.DefaultKind()
		if !ok {
			return usecase.TaskResult{}, usecase.NewErr(usecase.KindInvalid, "schema.no_default", errors.New("the schema has no default kind; give one with --kind"), "what", "kind")
		}
		t.Kind = k.Name
	}
	if t.Status == "" {
		st, ok := sc.DefaultStatus(schema.CategoryTodo)
		if !ok {
			return usecase.TaskResult{}, usecase.NewErr(usecase.KindInvalid, "schema.no_default", errors.New("the schema has no default todo status; give one with --status"), "what", "status")
		}
		t.Status = st.Name
	}
	if err := usecase.CheckPosition(in.Before, in.After); err != nil {
		return usecase.TaskResult{}, err
	}
	fromSibling := in.Parent == "" && (in.Before != "" || in.After != "")
	if fromSibling {
		x, err := ss.ResolveRef(ix, in.Before+in.After)
		if err != nil {
			return usecase.TaskResult{}, err
		}
		t.Workspace = x.Workspace
		if x.Parent != nil {
			p, ok := ix.Task(*x.Parent)
			if !ok {
				return usecase.TaskResult{}, usecase.NewErr(usecase.KindInvalid, "add.position", fmt.Errorf("the parent of %s is not in this store", usecase.RefName(ix, x.ULID)))
			}
			if err := usecase.LiveParent(ix, p); err != nil {
				return usecase.TaskResult{}, err
			}
			t.Parent = &p.ULID
		}
	}
	if in.Parent != "" {
		p, err := ss.ResolveRef(ix, in.Parent)
		if err != nil {
			return usecase.TaskResult{}, err
		}
		if err := usecase.LiveParent(ix, p); err != nil {
			return usecase.TaskResult{}, err
		}
		t.Parent = &p.ULID
		t.Workspace = p.Workspace
	}
	if in.Workspace != "" {
		w, err := usecase.ResolveWorkspace(ix, in.Workspace)
		if err != nil {
			return usecase.TaskResult{}, err
		}
		if fromSibling {
			// not the sibling's: RankAt says the sibling is not one
			t.Workspace = w.ULID
		} else if in.Parent != "" && w.ULID != t.Workspace {
			pw, ok := ix.Workspace(t.Workspace)
			if !ok {
				return usecase.TaskResult{}, usecase.NewErr(usecase.KindInvalid, "task.workspace_mismatch", fmt.Errorf("the parent's workspace %s is not in this store", t.Workspace))
			}
			return usecase.TaskResult{}, usecase.NewErr(usecase.KindInvalid, "task.workspace_mismatch", fmt.Errorf("the parent is in workspace %s", pw.Name))
		}
		t.Workspace = w.ULID
	} else if in.Parent == "" && !fromSibling {
		wss := ix.Workspaces()
		if len(wss) == 0 {
			return usecase.TaskResult{}, usecase.NewErr(usecase.KindConflict, "workspace.none", errors.New("the store has no workspace; add one with hedgehug workspace add <name> --prefix <PREFIX>"))
		}
		if len(wss) != 1 {
			names := make([]string, len(wss))
			for i, w := range wss {
				names[i] = fmt.Sprintf("%s (%s)", w.Name, w.IDPrefix)
			}
			// KindInvalid, not KindUsage: the CLI exits 1 here.
			return usecase.TaskResult{}, usecase.NewErr(usecase.KindInvalid, "workspace.choose", errors.New("there are several workspaces; give one with --workspace: "+strings.Join(names, ", ")), "names", names)
		}
		t.Workspace = wss[0].ULID
	}
	for _, as := range in.Assigns {
		if err := usecase.ApplyAssign(t, as, sc, ss.ResolveULID(ix)); err != nil {
			return usecase.TaskResult{}, err
		}
	}
	if in.FirstRepo != "" && !slices.Contains(t.Repos, in.FirstRepo) {
		t.Repos = append([]string{in.FirstRepo}, t.Repos...)
	}
	if t.Rank, err = ss.RankAt(ix, t.ULID, t.Parent, t.Workspace, in.Before, in.After, "add.position"); err != nil {
		return usecase.TaskResult{}, err
	}
	if t.CreatedBy, err = ss.AccountID(); err != nil {
		return usecase.TaskResult{}, err
	}
	if t.CreatedBy != "" {
		usecase.AddWatchers(t, t.CreatedBy)
	}
	watchNewAssignees(t, nil)
	hash, err := ss.UpdateCommit(s, store.Op{Name: "add", Task: t.ULID}, func(tx *store.Tx) error { return tx.PutTask(t) })
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, t.ULID, hash), nil
}

// Set applies assigns to the task arg names.
func Set(ss *usecase.Session, arg string, assigns []usecase.Assign) (usecase.TaskResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if ts != ss {
		return Set(ts, arg, assigns)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return usecase.TaskResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "set", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		assignees := slices.Clone(t.Assignees)
		for _, as := range assigns {
			if err := usecase.ApplyAssign(t, as, ix.Schema(), ss.ResolveULID(ix)); err != nil {
				return err
			}
		}
		watchNewAssignees(t, assignees)
		return tx.PutTask(t)
	})
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}

// SetStatus sets the status of the task arg names; an empty status means the
// default status of the done category.
func SetStatus(ss *usecase.Session, arg, name string) (usecase.TaskResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if ts != ss {
		return SetStatus(ts, arg, name)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	sc := ix.Schema()
	if name == "" {
		st, ok := sc.DefaultStatus(schema.CategoryDone)
		if !ok {
			return usecase.TaskResult{}, usecase.NewErr(usecase.KindInvalid, "schema.no_default", errors.New("the schema has no default done status"), "what", "done_status")
		}
		name = st.Name
	} else if _, ok := sc.Status(name); !ok {
		names := make([]string, len(sc.Statuses))
		for i, st := range sc.Statuses {
			names[i] = st.Name
		}
		return usecase.TaskResult{}, usecase.NewErr(usecase.KindInvalid, "status.unknown", fmt.Errorf("%q is not a status (statuses: %s)", name, strings.Join(names, ", ")), "name", name)
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return usecase.TaskResult{}, err
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
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}

// Dep makes the task arg names blocked by the task blockerArg names, or, with
// add false, no longer blocked by it.
func Dep(ss *usecase.Session, arg, blockerArg string, add bool) (usecase.TaskResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if ts != ss {
		return Dep(ts, arg, blockerArg, add)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return usecase.TaskResult{}, err
	}
	b, err := ss.ResolveRef(ix, blockerArg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "dep", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		has := slices.Contains(t.BlockedBy, b.ULID)
		switch {
		case add && has:
			return usecase.NewErr(usecase.KindInvalid, "dep.invalid", fmt.Errorf("%s is already blocked by %s", usecase.RefName(ix, e.ULID), usecase.RefName(ix, b.ULID)))
		case add:
			t.BlockedBy = append(t.BlockedBy, b.ULID)
		case !has:
			return usecase.NewErr(usecase.KindInvalid, "dep.invalid", fmt.Errorf("%s is not blocked by %s", usecase.RefName(ix, e.ULID), usecase.RefName(ix, b.ULID)))
		default:
			t.BlockedBy = slices.DeleteFunc(t.BlockedBy, func(x model.ULID) bool { return x == b.ULID })
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}

// Watch adds the writer of the store to the watchers of the task arg
// names, or, with on false, takes it out.
func Watch(ss *usecase.Session, arg string, on bool) (usecase.TaskResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if ts != ss {
		return Watch(ts, arg, on)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	acct, ok, err := ss.Writer()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if !ok {
		return usecase.TaskResult{}, usecase.ErrNoAccounts
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return usecase.TaskResult{}, err
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
			usecase.AddWatchers(t, acct.ID())
		} else {
			t.Watchers = slices.DeleteFunc(t.Watchers, func(w string) bool { return config.SameAccount(w, acct.ID()) })
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}

// watchNewAssignees adds to the watchers of t the assignees it has now and
// did not in before.
func watchNewAssignees(t *model.Task, before []string) {
	for _, as := range t.Assignees {
		if !slices.ContainsFunc(before, func(b string) bool { return config.SameAccount(b, as) }) {
			usecase.AddWatchers(t, as)
		}
	}
}
