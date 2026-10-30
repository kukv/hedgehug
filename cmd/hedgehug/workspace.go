package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// workspaceJSON is a workspace in --json output.
type workspaceJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	KeyPrefix string    `json:"key_prefix"`
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
				return usageErrorf("--prefix is required: the key prefix of the workspace, such as WEB")
			}
			return a.workspaceAdd(args[0], prefix)
		},
	}
	add.Flags().StringVar(&prefix, "prefix", "", "the key prefix of the workspace, such as WEB")
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
			Short: "Change the key prefix of a workspace and renumber its tasks",
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

// resolveWorkspace finds the workspace arg names.
func resolveWorkspace(ix *index.Index, arg string) (*model.Workspace, error) {
	w, err := ix.ResolveWorkspace(arg)
	var ae *index.AmbiguousWorkspaceError
	if errors.As(err, &ae) {
		lines := make([]string, len(ae.Candidates))
		for i, c := range ae.Candidates {
			lines[i] = fmt.Sprintf("%s (%s, %s)", c.Name, c.KeyPrefix, c.ID)
		}
		return nil, &ambiguousError{input: arg, noun: "workspaces", lines: lines}
	}
	return w, err
}

// checkPrefixElsewhere refuses a key prefix that a workspace of another store
// of the config has. A store that cannot be opened is skipped with a warning.
func (a *app) checkPrefixElsewhere(prefix, self string) error {
	c, _, err := a.config()
	if err != nil {
		return err
	}
	for _, e := range c.Stores {
		if e.Name == self {
			continue
		}
		s, err := store.Open(e.Path)
		if err != nil {
			_, _ = fmt.Fprintf(a.stderr, "warning: store %s is not checked for key prefixes: %v\n", e.Name, err)
			continue
		}
		for w, err := range s.Workspaces() {
			if err != nil {
				continue
			}
			if strings.EqualFold(w.KeyPrefix, prefix) {
				return fmt.Errorf("key prefix %s is used by workspace %s of store %s", prefix, w.Name, e.Name)
			}
		}
	}
	return nil
}

// selectedStoreName returns the name of the store the command works on.
func (a *app) selectedStoreName() (string, error) {
	c, _, err := a.config()
	if err != nil {
		return "", err
	}
	e, err := c.Select(a.storeName)
	return e.Name, err
}

func (a *app) workspaceAdd(name, prefix string) error {
	if err := model.ValidWorkspaceName(name); err != nil {
		return usageError{err}
	}
	if err := model.ValidKeyPrefix(prefix); err != nil {
		return usageError{err}
	}
	self, err := a.selectedStoreName()
	if err != nil {
		return err
	}
	if err := a.checkPrefixElsewhere(prefix, self); err != nil {
		return err
	}
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	w := &model.Workspace{ID: model.NewID(), Name: name, KeyPrefix: prefix, PrefixSet: a.now()}
	op := workspaceOp(w.ID, "add %s (%s)", w.Name, w.KeyPrefix)
	if _, err := a.update(s, op, func(tx *store.Tx) error { return tx.PutWorkspace(w) }); err != nil {
		return err
	}
	return a.printWorkspace(ix, w, fmt.Sprintf("Added workspace %s (%s, %s)", w.Name, w.KeyPrefix, w.ID))
}

func (a *app) workspaceRename(arg, name string) error {
	if err := model.ValidWorkspaceName(name); err != nil {
		return usageError{err}
	}
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	ws, err := resolveWorkspace(ix, arg)
	if err != nil {
		return err
	}
	old := ws.Name
	var w *model.Workspace
	op := workspaceOp(ws.ID, "rename %s → %s (%s)", old, name, ws.KeyPrefix)
	if _, err := a.update(s, op, func(tx *store.Tx) error {
		var err error
		if w, err = tx.Workspace(ws.ID); err != nil {
			return err
		}
		w.Name = name
		return tx.PutWorkspace(w)
	}); err != nil {
		return err
	}
	return a.printWorkspace(ix, w, fmt.Sprintf("Renamed workspace %s to %s (%s, %s)", old, w.Name, w.KeyPrefix, w.ID))
}

func (a *app) workspacePrefix(arg, prefix string) error {
	if err := model.ValidKeyPrefix(prefix); err != nil {
		return usageError{err}
	}
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	ws, err := resolveWorkspace(ix, arg)
	if err != nil {
		return err
	}
	if ws.KeyPrefix == prefix {
		return a.printWorkspace(ix, ws, fmt.Sprintf("%s: %s already has the key prefix %s (%s)", verb(false, ""), ws.Name, ws.KeyPrefix, ws.ID))
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
	var oldKeys []string
	op := workspaceOp(ws.ID, "prefix %s %s → %s", ws.Name, ws.KeyPrefix, prefix)
	if _, err := a.update(s, op, func(tx *store.Tx) error {
		var err error
		if w, err = tx.Workspace(ws.ID); err != nil {
			return err
		}
		w.KeyPrefix = prefix
		w.PrefixSet = a.now()
		if err := tx.PutWorkspace(w); err != nil {
			return err
		}
		// Every task of the workspace, deleted ones too, in the order of
		// its old number; assignKeys numbers them in the order they are
		// put. Read under the lock, so a task added since is renumbered too.
		if err := ix.Refresh(); err != nil {
			return err
		}
		var ids []model.ID
		for _, e := range ix.Tasks() {
			if e.Workspace == ws.ID && e.Broken == "" {
				ids = append(ids, e.ID)
			}
		}
		number := func(id model.ID) int {
			e, _ := ix.Task(id)
			_, n, _ := model.ParseKey(e.Key)
			return n
		}
		slices.SortFunc(ids, func(x, y model.ID) int {
			return cmp.Or(cmp.Compare(number(x), number(y)), cmp.Compare(x.String(), y.String()))
		})
		for _, id := range ids {
			t, err := tx.Task(id)
			if err != nil {
				return err
			}
			oldKeys = append(oldKeys, t.Key)
			tasks = append(tasks, t)
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if a.jsonOut {
		return a.printWorkspace(ix, w, "")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Changed the key prefix of %s to %s (%s); renumbered %d tasks\n", w.Name, w.KeyPrefix, w.ID, len(tasks))
	for i, t := range tasks {
		fmt.Fprintf(&b, "  %s → %s\n", oldKeys[i], t.Key)
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

func (a *app) workspaceRm(arg string) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	ws, err := resolveWorkspace(ix, arg)
	if err != nil {
		return err
	}
	if n := countTasks(ix, ws.ID, true); n > 0 {
		return fmt.Errorf("workspace %s still has %d tasks; move them with hedgehug mv --workspace or purge them", ws.Name, n)
	}
	op := workspaceOp(ws.ID, "rm %s (%s)", ws.Name, ws.KeyPrefix)
	if _, err := a.update(s, op, func(tx *store.Tx) error { return tx.RemoveWorkspace(ws.ID) }); err != nil {
		return err
	}
	return a.printWorkspace(ix, ws, fmt.Sprintf("Removed workspace %s (%s, %s)", ws.Name, ws.KeyPrefix, ws.ID))
}

// workspaceOp is the operation of a workspace command on the workspace id.
// The subject says what changes, as the command read it before the lock.
func workspaceOp(id model.ID, format string, args ...any) store.Op {
	return store.Op{Name: "workspace", Detail: fmt.Sprintf(format, args...), Workspace: id}
}

func (a *app) workspaceLs() error {
	_, ix, err := a.open()
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
		rows[i] = []string{w.Name, w.KeyPrefix, w.ID.String(), fmt.Sprint(countTasks(ix, w.ID, false))}
	}
	return writeTable(a.stdout, []string{"NAME", "PREFIX", "ID", "TASKS"}, rows, nil)
}

// countTasks counts the tasks of a workspace: the ones not deleted, or all
// of them when withDeleted.
func countTasks(ix *index.Index, ws model.ID, withDeleted bool) int {
	n := 0
	for _, e := range ix.Tasks() {
		if e.Workspace == ws && (withDeleted || e.DeletedAt == nil) {
			n++
		}
	}
	return n
}

func workspaceToJSON(ix *index.Index, w *model.Workspace) workspaceJSON {
	return workspaceJSON{ID: w.ID.String(), Name: w.Name, KeyPrefix: w.KeyPrefix, PrefixSet: w.PrefixSet, Tasks: countTasks(ix, w.ID, false)}
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
