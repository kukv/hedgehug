package ops

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// WorkspaceView is a workspace as the CLI prints it with --json.
type WorkspaceView struct {
	ULID      string    `json:"ulid"`
	Name      string    `json:"name"`
	IDPrefix  string    `json:"id_prefix"`
	PrefixSet time.Time `json:"prefix_set"`
	Tasks     int       `json:"tasks"` // not deleted
}

// WorkspaceViewOf is w as ix counts its tasks.
func WorkspaceViewOf(ix *index.Index, w *model.Workspace) WorkspaceView {
	return WorkspaceView{ULID: w.ULID.String(), Name: w.Name, IDPrefix: w.IDPrefix, PrefixSet: w.PrefixSet, Tasks: CountTasks(ix, w.ULID, false)}
}

// IDChange is a task id that a change of the id prefix replaced.
type IDChange struct{ Old, New string }

// WorkspaceResult is what a workspace operation did. Index is the index the
// operation read, which WorkspacePrefix refreshes: only its task counts are
// meant to be read from it.
type WorkspaceResult struct {
	Index     *index.Index
	Workspace *model.Workspace
	Committed bool
	Commit    string     // hex hash of the commit made; "" when nothing was committed
	OldName   string     // WorkspaceRename: the name before
	IDs       []IDChange // WorkspacePrefix: the ids of the renumbered tasks
	// Renumbered is len(IDs).
	Renumbered int
}

// CheckPrefixElsewhere refuses an id prefix that a workspace of another store
// of the config has; self is the store that is not looked at. A store that
// cannot be opened is skipped with a warning.
func (ss *Session) CheckPrefixElsewhere(prefix, self string) error {
	c, _, err := ss.Config()
	if err != nil {
		return err
	}
	var others []store.NamedStore
	for _, e := range c.Stores {
		if e.Name != self {
			others = append(others, store.NamedStore{Name: e.Name, Path: e.Path})
		}
	}
	uses := store.PrefixUses(others, []string{prefix}, func(name string, err error) {
		ss.warn(fmt.Sprintf("store %s is not checked for id prefixes: %v", name, err))
	})
	if len(uses) > 0 {
		return newErr(KindConflict, "workspace.prefix_taken",
			fmt.Errorf("id prefix %s is used by workspace %s of store %s", prefix, uses[0].Workspace.Name, uses[0].Store), "prefix", prefix)
	}
	return nil
}

// selectedStoreName returns the name of the store the session works on.
func (ss *Session) selectedStoreName() (string, error) {
	_, e, err := ss.Entry()
	return e.Name, err
}

func badWorkspaceName(name string) error {
	if err := model.ValidWorkspaceName(name); err != nil {
		return newErr(KindUsage, "workspace.bad_name", err, "value", name)
	}
	return nil
}

func badIDPrefix(prefix string) error {
	if err := model.ValidIDPrefix(prefix); err != nil {
		return newErr(KindUsage, "workspace.bad_prefix", err, "value", prefix)
	}
	return nil
}

// workspaceOp is the operation of a workspace command on the workspace ulid.
// Its fn sets the detail of the subject with SetDetail, from the workspace
// as it reads it under the lock.
func workspaceOp(ulid model.ULID) store.Op {
	return store.Op{Name: "workspace", Workspace: ulid}
}

// WorkspaceAdd adds a workspace to the store.
func (ss *Session) WorkspaceAdd(name, prefix string) (WorkspaceResult, error) {
	if err := badWorkspaceName(name); err != nil {
		return WorkspaceResult{}, err
	}
	if err := badIDPrefix(prefix); err != nil {
		return WorkspaceResult{}, err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return WorkspaceResult{}, err
	}
	self, err := ss.selectedStoreName()
	if err != nil {
		return WorkspaceResult{}, err
	}
	if err := ss.CheckPrefixElsewhere(prefix, self); err != nil {
		return WorkspaceResult{}, err
	}
	w := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix, PrefixSet: ss.Now()}
	hash, err := ss.UpdateCommit(s, workspaceOp(w.ULID), func(tx *store.Tx) error {
		tx.SetDetail(fmt.Sprintf("add %s (%s)", w.Name, w.IDPrefix))
		return tx.PutWorkspace(w)
	})
	if err != nil {
		return WorkspaceResult{}, err
	}
	return WorkspaceResult{Index: ix, Workspace: w, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// WorkspaceRename renames the workspace arg names.
func (ss *Session) WorkspaceRename(arg, name string) (WorkspaceResult, error) {
	if err := badWorkspaceName(name); err != nil {
		return WorkspaceResult{}, err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return WorkspaceResult{}, err
	}
	ws, err := ResolveWorkspace(ix, arg)
	if err != nil {
		return WorkspaceResult{}, err
	}
	var old string
	var w *model.Workspace
	hash, err := ss.UpdateCommit(s, workspaceOp(ws.ULID), func(tx *store.Tx) error {
		var err error
		if w, err = tx.Workspace(ws.ULID); err != nil {
			return err
		}
		old = w.Name
		tx.SetDetail(fmt.Sprintf("rename %s → %s (%s)", old, name, w.IDPrefix))
		w.Name = name
		return tx.PutWorkspace(w)
	})
	if err != nil {
		return WorkspaceResult{}, err
	}
	return WorkspaceResult{Index: ix, Workspace: w, Committed: !hash.IsZero(), Commit: commitHex(hash), OldName: old}, nil
}

// WorkspacePrefix changes the id prefix of the workspace arg names and
// renumbers its tasks. When the workspace has the prefix already, nothing is
// written and Committed is false.
func (ss *Session) WorkspacePrefix(arg, prefix string) (WorkspaceResult, error) {
	if err := badIDPrefix(prefix); err != nil {
		return WorkspaceResult{}, err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return WorkspaceResult{}, err
	}
	ws, err := ResolveWorkspace(ix, arg)
	if err != nil {
		return WorkspaceResult{}, err
	}
	if ws.IDPrefix == prefix {
		return WorkspaceResult{Index: ix, Workspace: ws}, nil
	}
	self, err := ss.selectedStoreName()
	if err != nil {
		return WorkspaceResult{}, err
	}
	if err := ss.CheckPrefixElsewhere(prefix, self); err != nil {
		return WorkspaceResult{}, err
	}
	var w *model.Workspace
	var tasks []*model.Task
	var oldIDs []string
	hash, err := ss.UpdateCommit(s, workspaceOp(ws.ULID), func(tx *store.Tx) error {
		var err error
		if w, err = tx.Workspace(ws.ULID); err != nil {
			return err
		}
		// Another process may have set this prefix since the check above.
		if w.IDPrefix == prefix {
			return nil
		}
		tx.SetDetail(fmt.Sprintf("prefix %s %s → %s", w.Name, w.IDPrefix, prefix))
		w.IDPrefix = prefix
		w.PrefixSet = ss.Now()
		if err := tx.PutWorkspace(w); err != nil {
			return err
		}
		// Every task of the workspace, deleted ones too, in the order of
		// its old number; assignIDs numbers them in the order they are
		// put. Read under the lock, so a task added since is renumbered too.
		if err := ix.Refresh(); err != nil {
			return err
		}
		var ulids []model.ULID
		for _, e := range ix.Tasks() {
			if e.Workspace == ws.ULID && e.Broken == "" {
				ulids = append(ulids, e.ULID)
			}
		}
		number := func(ulid model.ULID) int {
			e, _ := ix.Task(ulid)
			_, n, _ := model.ParseTaskID(e.ID)
			return n
		}
		slices.SortFunc(ulids, func(x, y model.ULID) int {
			return cmp.Or(cmp.Compare(number(x), number(y)), cmp.Compare(x.String(), y.String()))
		})
		for _, ulid := range ulids {
			t, err := tx.Task(ulid)
			if err != nil {
				return err
			}
			oldIDs = append(oldIDs, t.ID)
			tasks = append(tasks, t)
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return WorkspaceResult{}, err
	}
	r := WorkspaceResult{Index: ix, Workspace: w, Committed: !hash.IsZero(), Commit: commitHex(hash), Renumbered: len(tasks)}
	for i, t := range tasks {
		r.IDs = append(r.IDs, IDChange{Old: oldIDs[i], New: t.ID})
	}
	return r, nil
}

// WorkspaceRemove removes the workspace arg names, which has no tasks.
func (ss *Session) WorkspaceRemove(arg string) (WorkspaceResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return WorkspaceResult{}, err
	}
	ws, err := ResolveWorkspace(ix, arg)
	if err != nil {
		return WorkspaceResult{}, err
	}
	if n := CountTasks(ix, ws.ULID, true); n > 0 {
		return WorkspaceResult{}, newErr(KindConflict, "workspace.not_empty",
			fmt.Errorf("workspace %s still has %d tasks; move them with hedgehug mv --workspace or purge them", ws.Name, n))
	}
	w := ws
	hash, err := ss.UpdateCommit(s, workspaceOp(ws.ULID), func(tx *store.Tx) error {
		var err error
		if w, err = tx.Workspace(ws.ULID); err != nil {
			return err
		}
		tx.SetDetail(fmt.Sprintf("rm %s (%s)", w.Name, w.IDPrefix))
		return tx.RemoveWorkspace(ws.ULID)
	})
	if err != nil {
		return WorkspaceResult{}, err
	}
	return WorkspaceResult{Index: ix, Workspace: w, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// WorkspaceList lists the workspaces of the store, by name, with the number
// of their tasks that are not deleted.
func (ss *Session) WorkspaceList() (*index.Index, []WorkspaceView, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return nil, nil, err
	}
	wss := ix.Workspaces()
	out := make([]WorkspaceView, len(wss))
	for i, w := range wss {
		out[i] = WorkspaceViewOf(ix, w)
	}
	return ix, out, nil
}
