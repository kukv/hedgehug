package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/cli/prompt"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) mvCmd() *cobra.Command {
	var parent, before, after, workspace, to, scoped string
	var root, yes bool
	cmd := &cobra.Command{
		Use: "mv <id> [--workspace <ws>] [--parent <id> | --root] [--before <id> | --after <id>]\n" +
			"  hedgehug mv <id> --store <dest> [--workspace <ws>] [--parent <id>] [--scoped carry|drop] [--yes]",
		Short: "Move a task under another parent or among its siblings, or to another store",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := usecase.CheckPosition(before, after); err != nil {
				return err
			}
			if to != "" {
				// Here --store is the destination; the task is looked up in every store.
				if root || before != "" || after != "" {
					return usageErrorf("--store cannot be used with --root, --before or --after")
				}
				return a.mvStore(stores.MoveStoreInput{Arg: args[0], To: to, Workspace: workspace, Parent: parent, Scoped: scoped, Yes: yes})
			}
			if scoped != "" || yes {
				return usageErrorf("--scoped and --yes are for --store")
			}
			switch {
			case parent != "" && root:
				return usageErrorf("give --parent or --root, not both")
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
	f.StringVar(&to, "store", "", "move it with its subtree to this store, leaving a forward file behind")
	f.StringVar(&scoped, "scoped", "", "with --store: carry or drop the scoped schema of a project moved")
	f.BoolVar(&yes, "yes", false, "with --store: do not ask for confirmation")
	f.StringVar(&before, "before", "", "put it just before this sibling")
	f.StringVar(&after, "after", "", "put it just after this sibling")
	return cmd
}

func (a *app) mv(arg, workspaceArg, parentArg string, root bool, before, after string) error {
	res, err := tasks.Move(a.ss, tasks.MoveInput{Arg: arg, Workspace: workspaceArg, Parent: parentArg, Root: root, Before: before, After: after})
	if err != nil {
		return err
	}
	w, err := res.Write()
	if err != nil {
		return err
	}
	if err := a.printWrite(w, verb(res.Committed, "Moved")); err != nil || a.jsonOut || !res.Moved {
		return err
	}
	var b strings.Builder
	for _, c := range res.Renumbered() {
		fmt.Fprintf(&b, "  %s → %s\n", c.Old, c.New)
	}
	for _, d := range w.Detached {
		if d.Sprint != "" {
			fmt.Fprintf(&b, "  %s: removed from sprint %s\n", d.DisplayID, d.Sprint)
		}
		for _, r := range d.Releases {
			fmt.Fprintf(&b, "  %s: removed from release %s\n", d.DisplayID, r)
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// mvStore moves a task to another store. On a terminal it asks what the
// operation lacks: the workspace, --scoped, and the confirmation.
func (a *app) mvStore(in stores.MoveStoreInput) error {
	var res stores.MoveStoreResult
	var err error
	shown := false // the plan was shown, with the scoped lines
	for range 4 {
		if res, err = stores.MoveStore(a.ss, in); err == nil {
			return a.printMoveStore(res, shown)
		}
		e := usecase.AsError(err)
		if e == nil || !a.ask().Interactive() {
			return err
		}
		switch {
		case e.Code == "choice.required" && e.Args["what"] == "--workspace":
			names, _ := e.Args["candidates"].([]string)
			if in.Workspace, err = a.ask().Choose(prompt.Choose{Question: "Which workspace of " + in.To + "?", Options: names, Canceled: "canceled"}); err != nil {
				return err
			}
		case e.Code == "choice.required" && e.Args["what"] == "--scoped":
			projects, _ := e.Args["projects"].([]string)
			if in.Scoped, err = a.askScoped(projects); err != nil {
				return err
			}
		case e.Code == "confirm.required":
			var ce *stores.MoveConfirmError
			if !errors.As(err, &ce) {
				return err
			}
			if err = a.confirmMove(ce.Plan); err != nil {
				return err
			}
			in.Yes, in.Confirmed, shown = true, &ce.Plan, true
		default:
			return err
		}
	}
	return err
}

// askScoped asks whether the scoped schema of the projects moved goes along.
func (a *app) askScoped(projects []string) (string, error) {
	return a.ask().Choose(prompt.Choose{
		Question: "The scoped schema of " + strings.Join(projects, ", ") + " — ",
		Options:  []string{"carry", "drop"}, ByWord: true, Canceled: "Canceled",
	})
}

// confirmMove shows what a move cuts and says about the history (D8), and
// asks.
func (a *app) confirmMove(p stores.MovePlan) error {
	v := p.View()
	var b strings.Builder
	for _, l := range v.Unlinked {
		b.WriteString(l + "\n")
	}
	for _, l := range v.Scoped {
		b.WriteString(l + "\n")
	}
	fmt.Fprintf(&b, "The content before the move stays in the git history of %[1]s, and anyone who can read %[1]s can see it\n", v.From)
	if len(v.Hidden) > 0 {
		fmt.Fprintf(&b, "No longer visible to: %s\n", strings.Join(v.Hidden, ", "))
	}
	fmt.Fprintf(&b, "Move %s to %s? [y/N] ", plural.Count(len(v.Tasks), "task"), v.To)
	_, err := a.ask().Confirm(prompt.YesNo{Question: b.String(), Canceled: "Canceled", Refuse: "Canceled"})
	return err
}

// printMoveStore prints a move: a line for each task, or JSON. The scoped
// lines come with them unless the confirmation showed them.
func (a *app) printMoveStore(res stores.MoveStoreResult, shown bool) error {
	v := res.View()
	if a.jsonOut {
		return a.printJSON(v)
	}
	var b strings.Builder
	if res.Resumed {
		// The destination was written by an earlier run that stopped; this
		// one only finished the source.
		fmt.Fprintf(&b, "Finished the move: %s was left unfinished last time\n", v.From)
	}
	for _, m := range v.Tasks {
		fmt.Fprintf(&b, "Moved %s (%s) → %s %s\n", m.OldID, m.ULID, v.To, m.NewID)
	}
	if !shown {
		for _, l := range v.ScopedLines {
			b.WriteString(l + "\n")
		}
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}
