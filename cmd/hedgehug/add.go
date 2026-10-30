package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) addCmd() *cobra.Command {
	var parent, kind, status, workspace string
	var props []string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a task",
		Args:  minArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			assigns, err := parseAssigns(props)
			if err != nil {
				return err
			}
			return a.add(strings.Join(args, " "), parent, workspace, kind, status, assigns)
		},
	}
	f := cmd.Flags()
	f.StringVar(&parent, "parent", "", "the parent task")
	f.StringVar(&kind, "kind", "", "the kind (default: the schema's default kind)")
	f.StringVar(&status, "status", "", "the status (default: the default status of the todo category)")
	f.StringVar(&workspace, "workspace", "", "the workspace (default: the parent's, or the only one)")
	f.StringArrayVarP(&props, "prop", "p", nil, "set a field or a property, as in hedgehug set (repeatable)")
	return cmd
}

func (a *app) add(title, parentArg, workspaceArg, kind, status string, assigns []assign) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	sc := ix.Schema()
	t := &model.Task{ID: model.NewID(), Title: title, Kind: kind, Status: status}
	if t.Kind == "" {
		k, ok := sc.DefaultKind()
		if !ok {
			return errors.New("the schema has no default kind; give one with --kind")
		}
		t.Kind = k.Name
	}
	if t.Status == "" {
		st, ok := sc.DefaultStatus(schema.CategoryTodo)
		if !ok {
			return errors.New("the schema has no default todo status; give one with --status")
		}
		t.Status = st.Name
	}
	if parentArg != "" {
		p, err := a.resolve(ix, parentArg)
		if err != nil {
			return err
		}
		if err := liveParent(ix, p); err != nil {
			return err
		}
		t.Parent = &p.ID
		t.Workspace = p.Workspace
	}
	if workspaceArg != "" {
		w, err := resolveWorkspace(ix, workspaceArg)
		if err != nil {
			return err
		}
		if parentArg != "" && w.ID != t.Workspace {
			pw, _ := ix.Workspace(t.Workspace)
			return fmt.Errorf("the parent is in workspace %s", pw.Name)
		}
		t.Workspace = w.ID
	} else if parentArg == "" {
		wss := ix.Workspaces()
		if len(wss) != 1 {
			names := make([]string, len(wss))
			for i, w := range wss {
				names[i] = fmt.Sprintf("%s (%s)", w.Name, w.KeyPrefix)
			}
			return errors.New("there are several workspaces; give one with --workspace: " + strings.Join(names, ", "))
		}
		t.Workspace = wss[0].ID
	}
	for _, as := range assigns {
		if err := applyAssign(t, as, sc, a.resolveID(ix)); err != nil {
			return err
		}
	}
	if t.Rank, err = endRank(ix, t.Parent, t.ID); err != nil {
		return err
	}
	committed, err := a.update(s, store.Op{Name: "add", Task: t.ID}, func(tx *store.Tx) error { return tx.PutTask(t) })
	if err != nil {
		return err
	}
	return a.printTask(ix, t.ID, verb(committed, "Added"))
}

// siblings returns the tasks under parent (the root when nil) that are not
// deleted, leaving out skip, by rank.
func siblings(ix *index.Index, parent *model.ID, skip model.ID) []*index.Entry {
	var all []*index.Entry
	if parent == nil {
		for _, e := range ix.Roots() {
			if e.Parent == nil {
				all = append(all, e)
			}
		}
	} else {
		all = ix.Children(*parent)
	}
	out := all[:0:0]
	for _, e := range all {
		if e.DeletedAt == nil && e.ID != skip {
			out = append(out, e)
		}
	}
	return out
}

// endRank returns a rank after every sibling under parent.
func endRank(ix *index.Index, parent *model.ID, skip model.ID) (model.Rank, error) {
	sib := siblings(ix, parent, skip)
	var last model.Rank
	if len(sib) > 0 {
		last = sib[len(sib)-1].Rank
	}
	return model.Between(last, "")
}
