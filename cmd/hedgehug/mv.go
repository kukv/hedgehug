package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) mvCmd() *cobra.Command {
	var parent, before, after, workspace string
	var root bool
	cmd := &cobra.Command{
		Use:   "mv <id> [--workspace <ws>] [--parent <id> | --root] [--before <id> | --after <id>]",
		Short: "Move a task under another parent or among its siblings",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			switch {
			case parent != "" && root:
				return usageErrorf("give --parent or --root, not both")
			case before != "" && after != "":
				return usageErrorf("give --before or --after, not both")
			case parent == "" && !root && before == "" && after == "" && workspace == "":
				return usageErrorf("say where to: --workspace, --parent, --root, --before or --after")
			}
			return a.mv(args[0], workspace, parent, root, before, after)
		},
	}
	f := cmd.Flags()
	f.StringVar(&workspace, "workspace", "", "move it with its subtree to this workspace and renumber")
	f.StringVar(&parent, "parent", "", "the new parent")
	f.BoolVar(&root, "root", false, "move to the top level")
	f.StringVar(&before, "before", "", "put it just before this sibling")
	f.StringVar(&after, "after", "", "put it just after this sibling")
	return cmd
}

func (a *app) mv(arg, workspaceArg, parentArg string, root bool, before, after string) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	if err := notDeleted(ix, e); err != nil {
		return err
	}
	parent := e.Parent
	dest := e.Workspace
	switch {
	case root:
		parent = nil
	case parentArg != "":
		p, err := a.resolve(ix, parentArg)
		if err != nil {
			return err
		}
		if err := liveParent(ix, p); err != nil {
			return err
		}
		parent = &p.ID
		dest = p.Workspace
	}
	if workspaceArg != "" {
		w, err := resolveWorkspace(ix, workspaceArg)
		if err != nil {
			return err
		}
		switch {
		case parentArg != "" && w.ID != dest:
			pw, ok := ix.Workspace(dest)
			if !ok {
				return fmt.Errorf("the parent's workspace %s is not in this store", dest)
			}
			return fmt.Errorf("the parent is in workspace %s", pw.Name)
		case parentArg == "" && w.ID != e.Workspace:
			// The old parent stays behind, so the task goes to the top level.
			parent = nil
		}
		dest = w.ID
	}
	rank, err := a.rankAt(ix, e.ID, parent, before, after)
	if err != nil {
		return err
	}
	// A task and its subtree are in one workspace, so they move together.
	targets := []*index.Entry{e}
	if dest != e.Workspace {
		targets = append(targets, descendants(ix, e.ID, func(*index.Entry) bool { return true })...)
	}
	oldKeys := make([]string, len(targets))
	for i, x := range targets {
		oldKeys[i] = x.Key
	}
	committed, err := a.update(s, store.Op{Name: "mv", Task: e.ID}, func(tx *store.Tx) error {
		for _, x := range targets {
			t, err := tx.Task(x.ID)
			if err != nil {
				return err
			}
			t.Workspace = dest
			if x.ID == e.ID {
				t.Parent, t.Rank = parent, rank
			}
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := a.printTask(ix, e.ID, verb(committed, "Moved")); err != nil || a.jsonOut || len(targets) == 1 {
		return err
	}
	var b strings.Builder
	for i, x := range targets {
		n, _ := ix.Task(x.ID)
		fmt.Fprintf(&b, "  %s → %s\n", oldKeys[i], n.Key)
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// rankAt returns the rank for id under parent: just before or after a
// sibling, or after every sibling.
func (a *app) rankAt(ix *index.Index, id model.ID, parent *model.ID, before, after string) (model.Rank, error) {
	if before == "" && after == "" {
		return endRank(ix, parent, id)
	}
	ref := before
	if ref == "" {
		ref = after
	}
	x, err := a.resolve(ix, ref)
	if err != nil {
		return "", err
	}
	sib := siblings(ix, parent, id)
	i := slices.IndexFunc(sib, func(e *index.Entry) bool { return e.ID == x.ID })
	if i < 0 {
		return "", fmt.Errorf("%s is not under the same parent", refName(ix, x.ID))
	}
	if before != "" {
		var prev model.Rank
		if i > 0 {
			prev = sib[i-1].Rank
		}
		return model.Between(prev, x.Rank)
	}
	var next model.Rank
	if i+1 < len(sib) {
		next = sib[i+1].Rank
	}
	return model.Between(x.Rank, next)
}
