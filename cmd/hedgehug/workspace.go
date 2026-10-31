package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// workspaceJSON is a workspace in --json output.
type workspaceJSON struct {
	ULID      string    `json:"ulid"`
	Name      string    `json:"name"`
	IDPrefix  string    `json:"id_prefix"`
	PrefixSet time.Time `json:"prefix_set"`
	Tasks     int       `json:"tasks"` // not deleted
}

func (a *app) workspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage the workspaces of a store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	var prefix string
	add := &cobra.Command{
		Use:   "add <name> --prefix <P>",
		Short: "Add a workspace",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if prefix == "" {
				return usageErrorf("--prefix is required: the id prefix of the workspace, such as WEB")
			}
			return a.workspaceAdd(args[0], prefix)
		},
	}
	add.Flags().StringVar(&prefix, "prefix", "", "the id prefix of the workspace, such as WEB")
	cmd.AddCommand(add,
		&cobra.Command{
			Use:   "ls",
			Short: "List the workspaces",
			Args:  cobra.NoArgs,
			RunE:  func(_ *cobra.Command, _ []string) error { return a.workspaceLs() },
		},
		&cobra.Command{
			Use:   "rename <workspace> <name>",
			Short: "Rename a workspace",
			Args:  exactArgs(2),
			RunE:  func(_ *cobra.Command, args []string) error { return a.workspaceRename(args[0], args[1]) },
		},
		&cobra.Command{
			Use:   "prefix <workspace> <P>",
			Short: "Change the id prefix of a workspace and renumber its tasks",
			Args:  exactArgs(2),
			RunE:  func(_ *cobra.Command, args []string) error { return a.workspacePrefix(args[0], args[1]) },
		},
		&cobra.Command{
			Use:   "rm <workspace>",
			Short: "Remove a workspace that has no tasks",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.workspaceRm(args[0]) },
		},
	)
	return cmd
}

// checkPrefixElsewhere refuses an id prefix that a workspace of another store
// of the config has. A store that cannot be opened is skipped with a warning.
func (a *app) checkPrefixElsewhere(prefix, self string) error {
	c, _, err := a.ss.Config()
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
		_, _ = fmt.Fprintf(a.stderr, "warning: store %s is not checked for id prefixes: %v\n", name, err)
	})
	if len(uses) > 0 {
		return fmt.Errorf("id prefix %s is used by workspace %s of store %s", prefix, uses[0].Workspace.Name, uses[0].Store)
	}
	return nil
}

// selectedStoreName returns the name of the store the command works on.
func (a *app) selectedStoreName() (string, error) {
	c, _, err := a.ss.Config()
	if err != nil {
		return "", err
	}
	e, err := c.Select(a.ss.StoreName)
	return e.Name, err
}

func (a *app) workspaceAdd(name, prefix string) error {
	if err := model.ValidWorkspaceName(name); err != nil {
		return usageError{err}
	}
	if err := model.ValidIDPrefix(prefix); err != nil {
		return usageError{err}
	}
	self, err := a.selectedStoreName()
	if err != nil {
		return err
	}
	if err := a.checkPrefixElsewhere(prefix, self); err != nil {
		return err
	}
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	w := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix, PrefixSet: a.now()}
	if _, err := a.ss.Update(s, workspaceOp(w.ULID), func(tx *store.Tx) error {
		tx.SetDetail(fmt.Sprintf("add %s (%s)", w.Name, w.IDPrefix))
		return tx.PutWorkspace(w)
	}); err != nil {
		return err
	}
	return a.printWorkspace(ix, w, fmt.Sprintf("Added workspace %s (%s, %s)", w.Name, w.IDPrefix, w.ULID))
}

func (a *app) workspaceRename(arg, name string) error {
	if err := model.ValidWorkspaceName(name); err != nil {
		return usageError{err}
	}
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	ws, err := ops.ResolveWorkspace(ix, arg)
	if err != nil {
		return err
	}
	var old string
	var w *model.Workspace
	if _, err := a.ss.Update(s, workspaceOp(ws.ULID), func(tx *store.Tx) error {
		var err error
		if w, err = tx.Workspace(ws.ULID); err != nil {
			return err
		}
		old = w.Name
		tx.SetDetail(fmt.Sprintf("rename %s → %s (%s)", old, name, w.IDPrefix))
		w.Name = name
		return tx.PutWorkspace(w)
	}); err != nil {
		return err
	}
	return a.printWorkspace(ix, w, fmt.Sprintf("Renamed workspace %s to %s (%s, %s)", old, w.Name, w.IDPrefix, w.ULID))
}

func (a *app) workspacePrefix(arg, prefix string) error {
	if err := model.ValidIDPrefix(prefix); err != nil {
		return usageError{err}
	}
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	ws, err := ops.ResolveWorkspace(ix, arg)
	if err != nil {
		return err
	}
	if ws.IDPrefix == prefix {
		return a.printWorkspace(ix, ws, fmt.Sprintf("%s: %s already has the id prefix %s (%s)", verb(false, ""), ws.Name, ws.IDPrefix, ws.ULID))
	}
	self, err := a.selectedStoreName()
	if err != nil {
		return err
	}
	if err := a.checkPrefixElsewhere(prefix, self); err != nil {
		return err
	}
	var w *model.Workspace
	var tasks []*model.Task
	var oldIDs []string
	committed, err := a.ss.Update(s, workspaceOp(ws.ULID), func(tx *store.Tx) error {
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
		w.PrefixSet = a.now()
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
		return err
	}
	if !committed {
		return a.printWorkspace(ix, w, fmt.Sprintf("%s: %s already has the id prefix %s (%s)", verb(false, ""), w.Name, w.IDPrefix, w.ULID))
	}
	if a.jsonOut {
		return a.printWorkspace(ix, w, "")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Changed the id prefix of %s to %s (%s); renumbered %d tasks\n", w.Name, w.IDPrefix, w.ULID, len(tasks))
	for i, t := range tasks {
		fmt.Fprintf(&b, "  %s → %s\n", oldIDs[i], t.ID)
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

func (a *app) workspaceRm(arg string) error {
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	ws, err := ops.ResolveWorkspace(ix, arg)
	if err != nil {
		return err
	}
	if n := countTasks(ix, ws.ULID, true); n > 0 {
		return fmt.Errorf("workspace %s still has %d tasks; move them with hedgehug mv --workspace or purge them", ws.Name, n)
	}
	w := ws
	if _, err := a.ss.Update(s, workspaceOp(ws.ULID), func(tx *store.Tx) error {
		var err error
		if w, err = tx.Workspace(ws.ULID); err != nil {
			return err
		}
		tx.SetDetail(fmt.Sprintf("rm %s (%s)", w.Name, w.IDPrefix))
		return tx.RemoveWorkspace(ws.ULID)
	}); err != nil {
		return err
	}
	return a.printWorkspace(ix, w, fmt.Sprintf("Removed workspace %s (%s, %s)", w.Name, w.IDPrefix, w.ULID))
}

// workspaceOp is the operation of a workspace command on the workspace ulid.
// Its fn sets the detail of the subject with SetDetail, from the workspace
// as it reads it under the lock.
func workspaceOp(ulid model.ULID) store.Op {
	return store.Op{Name: "workspace", Workspace: ulid}
}

func (a *app) workspaceLs() error {
	_, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	wss := ix.Workspaces()
	if a.jsonOut {
		out := make([]workspaceJSON, len(wss))
		for i, w := range wss {
			out[i] = workspaceToJSON(ix, w)
		}
		return a.printJSON(out)
	}
	rows := make([][]string, len(wss))
	for i, w := range wss {
		rows[i] = []string{w.Name, w.IDPrefix, w.ULID.String(), fmt.Sprint(countTasks(ix, w.ULID, false))}
	}
	return writeTable(a.stdout, []string{"NAME", "PREFIX", "ULID", "TASKS"}, rows, nil)
}

// countTasks counts the tasks of a workspace: the ones not deleted, or all
// of them when withDeleted.
func countTasks(ix *index.Index, ws model.ULID, withDeleted bool) int {
	n := 0
	for _, e := range ix.Tasks() {
		if e.Workspace == ws && (withDeleted || e.DeletedAt == nil) {
			n++
		}
	}
	return n
}

func workspaceToJSON(ix *index.Index, w *model.Workspace) workspaceJSON {
	return workspaceJSON{ULID: w.ULID.String(), Name: w.Name, IDPrefix: w.IDPrefix, PrefixSet: w.PrefixSet, Tasks: countTasks(ix, w.ULID, false)}
}

// printWorkspace prints w as JSON, or line as text. ix is the index from
// before the write; only its task counts are read.
func (a *app) printWorkspace(ix *index.Index, w *model.Workspace, line string) error {
	if a.jsonOut {
		return a.printJSON(workspaceToJSON(ix, w))
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
