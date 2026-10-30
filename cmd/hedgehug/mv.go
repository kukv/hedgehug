package main

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) mvCmd() *cobra.Command {
	var parent, before, after string
	var root bool
	cmd := &cobra.Command{
		Use:   "mv <id> [--parent <id> | --root] [--before <id> | --after <id>]",
		Short: "Move a task under another parent or among its siblings",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			switch {
			case parent != "" && root:
				return usageErrorf("give --parent or --root, not both")
			case before != "" && after != "":
				return usageErrorf("give --before or --after, not both")
			case parent == "" && !root && before == "" && after == "":
				return usageErrorf("say where to: --parent, --root, --before or --after")
			}
			return a.mv(args[0], parent, root, before, after)
		},
	}
	f := cmd.Flags()
	f.StringVar(&parent, "parent", "", "the new parent")
	f.BoolVar(&root, "root", false, "move to the top level")
	f.StringVar(&before, "before", "", "put it just before this sibling")
	f.StringVar(&after, "after", "", "put it just after this sibling")
	return cmd
}

func (a *app) mv(arg, parentArg string, root bool, before, after string) error {
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
	}
	rank, err := a.rankAt(ix, e.ID, parent, before, after)
	if err != nil {
		return err
	}
	committed, err := a.update(s, store.Op{Name: "mv", Task: e.ID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ID)
		if err != nil {
			return err
		}
		t.Parent, t.Rank = parent, rank
		return tx.PutTask(t)
	})
	if err != nil {
		return err
	}
	return a.printTask(ix, e.ID, verb(committed, "Moved"))
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
