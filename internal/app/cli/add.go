package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/cli/prompt"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) addCmd() *cobra.Command {
	var parent, project, kind, status, workspace, body, before, after, sprint, template string
	var withRepo, noTemplate bool
	var props, releases []string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a task",
		Long: `Add a task. --template expands a template with its child tasks, and the title
may be left out when the template has one. Without it, the default template
of the kind is expanded, unless --no-template.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if template != "" {
				return nil
			}
			return minArgs(1)(cmd, args)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if err := usecase.CheckPosition(before, after); err != nil {
				return err
			}
			assigns, err := usecase.ParseAssigns(props)
			if err != nil {
				return err
			}
			if sprint != "" {
				assigns = append(assigns, usecase.Assign{Key: "sprint", Op: "=", Value: sprint})
			}
			for _, r := range releases {
				assigns = append(assigns, usecase.Assign{Key: "releases", Op: "+=", Value: r})
			}
			return a.add(tasks.AddHereInput{
				Context: usecase.Input{Parent: parent, Project: project, Workspace: workspace},
				Repo:    withRepo,
				Task:    tasks.AddInput{Title: strings.Join(args, " "), Kind: kind, Status: status, Body: body, Assigns: assigns, Before: before, After: after, Template: template, NoTemplate: noTemplate},
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&parent, "parent", "", "the parent task")
	f.StringVar(&project, "project", "", "the project to add under (a task of kind project)")
	f.StringVar(&kind, "kind", "", "the kind (default: task)")
	f.StringVar(&status, "status", "", "the status (default: the default status of the todo category)")
	f.StringVar(&workspace, "workspace", "", "the workspace (default: the parent's, or the only one)")
	f.StringVar(&before, "before", "", "put it just before this sibling (without --parent, under the sibling's parent)")
	f.StringVar(&after, "after", "", "put it just after this sibling (without --parent, under the sibling's parent)")
	f.StringVar(&body, "body", "", "the body, in Markdown")
	f.BoolVar(&withRepo, "repo", false, "put the current repository on the task (the repo ID of its remote: origin, else another remote)")
	f.StringVar(&template, "template", "", "expand this template")
	f.BoolVar(&noTemplate, "no-template", false, "do not expand the default template of the kind")
	f.StringVar(&sprint, "sprint", "", "put it in this sprint of its workspace")
	f.StringArrayVar(&releases, "release", nil, "put it in this release of its workspace (repeatable)")
	f.StringArrayVarP(&props, "prop", "p", nil, "set a field or a property, as in hedgehug set (repeatable)")
	return cmd
}

// maxAsks is the most questions one add can ask: the branch, the project, the
// store and the workspace.
const maxAsks = 4

func (a *app) add(in tasks.AddHereInput) error {
	in.Context.Interactive = a.ask().Interactive()
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
			if err := a.printAdded(r.TaskResult); err != nil {
				return err
			}
			if a.jsonOut {
				return nil
			}
			_, err := fmt.Fprintln(a.stdout, "→ added to "+r.Destination)
			return err
		}
	}
	return errors.New("add asked too many questions")
}

// printAdded prints the task added, with what its template did.
func (a *app) printAdded(r usecase.TaskResult) error {
	w, err := r.Write()
	if err != nil {
		return err
	}
	if err := a.printWrite(w, verb(r.Committed, "Added")); err != nil || a.jsonOut {
		return err
	}
	var b strings.Builder
	if w.Children > 0 {
		fmt.Fprintf(&b, "  with %s under it\n", plural.Count(w.Children, "task"))
	}
	writeDetached(&b, w.Detached)
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// askBranchParent asks whether to add under the task the branch names.
func (a *app) askBranchParent(q *tasks.BranchQuestion) (bool, error) {
	return a.ask().Confirm(prompt.YesNo{
		Question: fmt.Sprintf("Add as a child of %s %s? [Y/n] ", q.ParentID, q.ParentTitle),
		Default:  true, Canceled: "canceled", Strict: true,
	})
}

// askPick lists the options of c on stderr and reads the number of the one to
// pick; Enter takes the default, if there is one.
func (a *app) askPick(c *usecase.Choice) (string, error) {
	promptWord := map[string]string{"project": "Project", "store": "Store", "workspace": "Workspace"}[c.What]
	return a.ask().Choose(prompt.Choose{Options: c.Options, Labels: c.Labels, Prompt: promptWord, Default: c.Default, Canceled: "canceled"})
}
