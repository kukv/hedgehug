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
		parent = &p.ULID
		dest = p.Workspace
	}
	if workspaceArg != "" {
		w, err := resolveWorkspace(ix, workspaceArg)
		if err != nil {
			return err
		}
		switch {
		case parentArg != "" && w.ULID != dest:
			pw, ok := ix.Workspace(dest)
			if !ok {
				return fmt.Errorf("the parent's workspace %s is not in this store", dest)
			}
			return fmt.Errorf("the parent is in workspace %s", pw.Name)
		case parentArg == "" && w.ULID != e.Workspace:
			// The old parent stays behind, so the task goes to the top level.
			parent = nil
		}
		dest = w.ULID
	}
	rank, err := a.rankAt(ix, e.ULID, parent, before, after)
	if err != nil {
		return err
	}
	// The tasks put; assignIDs gives them their new ids in place.
	var tasks []*model.Task
	var oldIDs []string
	committed, err := a.update(s, store.Op{Name: "mv", Task: e.ULID}, func(tx *store.Tx) error {
		// A task and its subtree are in one workspace, so they move
		// together. The subtree is read under the lock, so a child added
		// since the index was read moves too.
		ids := []model.ULID{e.ULID}
		if dest != e.Workspace {
			if err := ix.Refresh(); err != nil {
				return err
			}
			for _, d := range descendants(ix, e.ULID, func(*index.Entry) bool { return true }) {
				ids = append(ids, d.ULID)
			}
		}
		for _, id := range ids {
			t, err := tx.Task(id)
			if err != nil {
				return err
			}
			oldIDs = append(oldIDs, t.ID)
			t.Workspace = dest
			if id == e.ULID {
				t.Parent, t.Rank = parent, rank
			}
			if err := tx.PutTask(t); err != nil {
				return err
			}
			tasks = append(tasks, t)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := a.printTask(ix, e.ULID, verb(committed, "Moved")); err != nil || a.jsonOut || dest == e.Workspace {
		return err
	}
	var b strings.Builder
	for i, t := range tasks {
		fmt.Fprintf(&b, "  %s → %s\n", oldIDs[i], t.ID)
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// rankAt returns the rank for id under parent: just before or after a
// sibling, or after every sibling.
func (a *app) rankAt(ix *index.Index, id model.ULID, parent *model.ULID, before, after string) (model.Rank, error) {
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
	i := slices.IndexFunc(sib, func(e *index.Entry) bool { return e.ULID == x.ULID })
	if i < 0 {
		return "", fmt.Errorf("%s is not under the same parent", refName(ix, x.ULID))
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
