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
	var p *index.Entry
	if parentArg != "" {
		if p, err = a.resolve(ix, parentArg); err != nil {
			return err
		}
	}
	var w *model.Workspace
	if workspaceArg != "" {
		if w, err = resolveWorkspace(ix, workspaceArg); err != nil {
			return err
		}
	}
	if _, err := a.mvTargetOf(ix, e, p, w, root, before, after); err != nil {
		return err
	}
	// The tasks put; assignIDs gives them their new ids in place.
	var tasks []*model.Task
	var oldIDs []string
	moved := false // to another workspace
	committed, err := a.update(s, store.Op{Name: "mv", Task: e.ULID}, func(tx *store.Tx) error {
		// Work out the move again under the lock, from the tasks as they
		// are now: another process may have moved or deleted them since
		// the index was read.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		if p != nil {
			if p, err = current(ix, p); err != nil {
				return err
			}
		}
		to, err := a.mvTargetOf(ix, e, p, w, root, before, after)
		if err != nil {
			return err
		}
		// A task and its subtree are in one workspace, so they move
		// together, a child added since the index was read too.
		ulids := []model.ULID{e.ULID}
		if moved = to.workspace != e.Workspace; moved {
			for _, d := range descendants(ix, e.ULID, func(*index.Entry) bool { return true }) {
				ulids = append(ulids, d.ULID)
			}
		}
		for _, ulid := range ulids {
			t, err := tx.Task(ulid)
			if err != nil {
				return err
			}
			oldIDs = append(oldIDs, t.ID)
			t.Workspace = to.workspace
			if ulid == e.ULID {
				t.Parent, t.Rank = to.parent, to.rank
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
	if err := a.printTask(ix, e.ULID, verb(committed, "Moved")); err != nil || a.jsonOut || !moved {
		return err
	}
	var b strings.Builder
	for i, t := range tasks {
		fmt.Fprintf(&b, "  %s → %s\n", oldIDs[i], t.ID)
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// mvTarget is where mv puts a task.
type mvTarget struct {
	parent    *model.ULID
	workspace model.ULID
	rank      model.Rank
}

// mvTargetOf works out where mv puts e, as ix has the tasks: at the top
// level when root, or under the new parent p (nil to keep the parent), in
// the workspace w (nil to follow the parent), before or after a sibling.
func (a *app) mvTargetOf(ix *index.Index, e, p *index.Entry, w *model.Workspace, root bool, before, after string) (mvTarget, error) {
	if err := notDeleted(ix, e); err != nil {
		return mvTarget{}, err
	}
	to := mvTarget{parent: e.Parent, workspace: e.Workspace}
	switch {
	case root:
		to.parent = nil
	case p != nil:
		if err := liveParent(ix, p); err != nil {
			return mvTarget{}, err
		}
		to.parent, to.workspace = &p.ULID, p.Workspace
	}
	if w != nil {
		switch {
		case p != nil && w.ULID != to.workspace:
			pw, ok := ix.Workspace(to.workspace)
			if !ok {
				return mvTarget{}, fmt.Errorf("the parent's workspace %s is not in this store", to.workspace)
			}
			return mvTarget{}, fmt.Errorf("the parent is in workspace %s", pw.Name)
		case p == nil && w.ULID != e.Workspace:
			// The old parent stays behind, so the task goes to the top level.
			to.parent = nil
		}
		to.workspace = w.ULID
	}
	var err error
	to.rank, err = a.rankAt(ix, e.ULID, to.parent, before, after)
	return to, err
}

// rankAt returns the rank for ulid under parent: just before or after a
// sibling, or after every sibling.
func (a *app) rankAt(ix *index.Index, ulid model.ULID, parent *model.ULID, before, after string) (model.Rank, error) {
	if before == "" && after == "" {
		return endRank(ix, parent, ulid)
	}
	ref := before
	if ref == "" {
		ref = after
	}
	x, err := a.resolve(ix, ref)
	if err != nil {
		return "", err
	}
	sib := siblings(ix, parent, ulid)
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
