package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

func (a *app) addCmd() *cobra.Command {
	var parent, project, kind, status, workspace, body, before, after string
	var withRepo bool
	var props []string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a task",
		Args:  minArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := usecase.CheckPosition(before, after); err != nil {
				return err
			}
			assigns, err := usecase.ParseAssigns(props)
			if err != nil {
				return err
			}
			return a.add(tasks.AddHereInput{
				Context: usecase.Input{Parent: parent, Project: project, Workspace: workspace},
				Repo:    withRepo,
				Task:    tasks.AddInput{Title: strings.Join(args, " "), Kind: kind, Status: status, Body: body, Assigns: assigns, Before: before, After: after},
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&parent, "parent", "", "the parent task")
	f.StringVar(&project, "project", "", "the project to add under (a task of kind project)")
	f.StringVar(&kind, "kind", "", "the kind (default: the schema's default kind)")
	f.StringVar(&status, "status", "", "the status (default: the default status of the todo category)")
	f.StringVar(&workspace, "workspace", "", "the workspace (default: the parent's, or the only one)")
	f.StringVar(&before, "before", "", "put it just before this sibling (without --parent, under the sibling's parent)")
	f.StringVar(&after, "after", "", "put it just after this sibling (without --parent, under the sibling's parent)")
	f.StringVar(&body, "body", "", "the body, in Markdown")
	f.BoolVar(&withRepo, "repo", false, "put the current repository on the task (the repo ID of its remote: origin, else another remote)")
	f.StringArrayVarP(&props, "prop", "p", nil, "set a field or a property, as in hedgehug set (repeatable)")
	return cmd
}

// maxAsks is the most questions one add can ask: the branch, the project, the
// store and the workspace.
const maxAsks = 4

func (a *app) add(in tasks.AddHereInput) error {
	in.Context.Interactive = a.isTerminal(a.stdin)
	if a.getwd != nil {
		dir, err := a.getwd()
		if err != nil {
			return err
		}
		in.Context.Dir = dir
	}
	for range maxAsks + 1 {
		r, confirm, choice, err := tasks.AddHere(a.ss, in)
		if err != nil {
			return err
		}
		switch {
		case confirm != nil:
			yes, err := a.askBranchParent(confirm)
			if err != nil {
				return err
			}
			in.Context.BranchParent = usecase.No
			if yes {
				in.Context.BranchParent = usecase.Yes
			}
		case choice != nil:
			picked, err := a.askPick(choice)
			if err != nil {
				return err
			}
			switch choice.What {
			case "project":
				in.Context.PickProject = picked
			case "store":
				in.Context.PickStore = picked
			default:
				in.Context.PickWorkspace = picked
			}
		default:
			if err := a.printTask(r.Index, r.ULID, verb(r.Committed, "Added")); err != nil {
				return err
			}
			if a.jsonOut {
				return nil
			}
			_, err := fmt.Fprintln(a.stdout, "→ added to "+destinationLabel(r.Destination))
			return err
		}
	}
	return errors.New("add asked too many questions")
}

// destinationLabel is "<store> / <parent's title>", or the workspace's name
// for the root.
func destinationLabel(d *usecase.Destination) string {
	if d.Parent != nil {
		return d.Store.Name + " / " + d.Parent.Title
	}
	return d.Store.Name + " / " + d.Workspace.Name
}

// shownID is the id of e as its store shows it.
func shownID(m *mount.Mounted, e *index.Entry) (string, error) {
	id := e.ULID.String()
	err := m.With(func(ix *index.Index) {
		if e.ID != "" {
			id = ix.DisplayID(e)
		}
	})
	return id, err
}

// askBranchParent asks whether to add under the task the branch names.
func (a *app) askBranchParent(c *usecase.Confirm) (bool, error) {
	id, err := shownID(c.Store, c.Parent)
	if err != nil {
		return false, err
	}
	if _, err := fmt.Fprintf(a.stderr, "Add as a child of %s %s? [Y/n] ", id, c.Parent.Title); err != nil {
		return false, err
	}
	line, err := a.readLine()
	if err != nil && line == "" {
		return false, errors.New("canceled")
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	}
	return false, usageErrorf("%q is not y or n", strings.TrimSpace(line))
}

// askPick lists the options of c on stderr and reads the number of the one to
// pick; Enter takes the default, if there is one.
func (a *app) askPick(c *usecase.Choice) (string, error) {
	prompt := map[string]string{"project": "Project", "store": "Store", "workspace": "Workspace"}[c.What]
	return a.askChoice(c.Options, choice{labels: c.Labels, prompt: prompt, def: c.Default})
}
