package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
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
			if to != "" {
				// Here --store is the destination; the task is looked up in every store.
				if root || before != "" || after != "" {
					return usageErrorf("--store cannot be used with --root, --before or --after")
				}
				return a.mvStore(ops.MoveStoreInput{Arg: args[0], To: to, Workspace: workspace, Parent: parent, Scoped: scoped, Yes: yes})
			}
			if scoped != "" || yes {
				return usageErrorf("--scoped and --yes are for --store")
			}
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
	f.StringVar(&to, "store", "", "move it with its subtree to this store, leaving a forward file behind")
	f.StringVar(&scoped, "scoped", "", "with --store: carry or drop the scoped schema of a project moved")
	f.BoolVar(&yes, "yes", false, "with --store: do not ask for confirmation")
	f.StringVar(&before, "before", "", "put it just before this sibling")
	f.StringVar(&after, "after", "", "put it just after this sibling")
	return cmd
}

func (a *app) mv(arg, workspaceArg, parentArg string, root bool, before, after string) error {
	res, err := a.ss.Move(ops.MoveInput{Arg: arg, Workspace: workspaceArg, Parent: parentArg, Root: root, Before: before, After: after})
	if err != nil {
		return err
	}
	if a.jsonOut && len(res.Detached) > 0 {
		// The task as the other commands print it, and what the move took off
		// it, as the server tells it.
		if err := res.Index.Refresh(); err != nil {
			return err
		}
		e, ok := res.Index.Task(res.ULID)
		if !ok {
			return fmt.Errorf("task %s is not in the index after the write", res.ULID)
		}
		return a.printJSON(struct {
			*ops.TaskView
			Detached []ops.Detached `json:"detached"`
		}{ops.TaskViewOf(res.Index, e), res.Detached})
	}
	if err := a.printTask(res.Index, res.ULID, verb(res.Committed, "Moved")); err != nil || a.jsonOut || !res.Moved {
		return err
	}
	var b strings.Builder
	for i, t := range res.Tasks {
		fmt.Fprintf(&b, "  %s → %s\n", res.Index.QualifyID(res.OldIDs[i]), res.Index.QualifyID(t.ID))
	}
	for _, d := range res.Detached {
		id := res.Index.QualifyID(d.ID)
		if d.Sprint != "" {
			fmt.Fprintf(&b, "  %s: removed from sprint %s\n", id, d.Sprint)
		}
		for _, r := range d.Releases {
			fmt.Fprintf(&b, "  %s: removed from release %s\n", id, r)
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// mvStore moves a task to another store. On a terminal it asks what the
// operation lacks: the workspace, --scoped, and the confirmation.
func (a *app) mvStore(in ops.MoveStoreInput) error {
	var res ops.MoveStoreResult
	var err error
	shown := false // the plan was shown, with the scoped lines
	for range 4 {
		if res, err = a.ss.MoveStore(in); err == nil {
			return a.printMoveStore(res, shown)
		}
		e := ops.AsError(err)
		if e == nil || !a.isTerminal(a.stdin) {
			return err
		}
		switch {
		case e.Code == "choice.required" && e.Args["what"] == "--workspace":
			names, _ := e.Args["candidates"].([]string)
			if in.Workspace, err = a.askChoice(names, choice{question: "Which workspace of " + in.To + "?"}); err != nil {
				return err
			}
		case e.Code == "choice.required" && e.Args["what"] == "--scoped":
			projects, _ := e.Args["projects"].([]string)
			if in.Scoped, err = a.askScoped(projects); err != nil {
				return err
			}
		case e.Code == "confirm.required":
			var ce *ops.MoveConfirmError
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
	if _, err := fmt.Fprintf(a.stderr, "The scoped schema of %s — [c]arry / [d]rop: ", strings.Join(projects, ", ")); err != nil {
		return "", err
	}
	line, err := a.readLine()
	if err != nil && line == "" {
		return "", errors.New("Canceled")
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "c", "carry":
		return "carry", nil
	case "d", "drop":
		return "drop", nil
	}
	return "", usageErrorf("%q is not carry or drop", strings.TrimSpace(line))
}

// confirmMove shows what a move cuts and says about the history (D8), and
// asks.
func (a *app) confirmMove(p ops.MovePlan) error {
	v := p.View()
	var b strings.Builder
	for _, l := range v.Unlinked {
		b.WriteString(l + "\n")
	}
	for _, l := range v.Scoped {
		b.WriteString(l + "\n")
	}
	fmt.Fprintf(&b, "The content before the move stays in the git history of %[1]s, and anyone who can read %[1]s can see it; use hedgehug purge --rewrite-history to remove it\n", v.From)
	if len(v.Hidden) > 0 {
		fmt.Fprintf(&b, "No longer visible to: %s\n", strings.Join(v.Hidden, ", "))
	}
	fmt.Fprintf(&b, "Move %d tasks to %s? [y/N] ", len(v.Tasks), v.To)
	if _, err := fmt.Fprint(a.stderr, b.String()); err != nil {
		return err
	}
	line, err := a.readLine()
	if err != nil && line == "" {
		return errors.New("Canceled")
	}
	if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
		return errors.New("Canceled")
	}
	return nil
}

// printMoveStore prints a move: a line for each task, or JSON. The scoped
// lines come with them unless the confirmation showed them.
func (a *app) printMoveStore(res ops.MoveStoreResult, shown bool) error {
	p := res.Plan
	if a.jsonOut {
		type taskJSON struct {
			ULID  string `json:"ulid"`
			OldID string `json:"old_id"`
			NewID string `json:"new_id"`
		}
		type unlinkJSON struct {
			Task   string `json:"task"`
			Field  string `json:"field"`
			Target string `json:"target"`
		}
		type scopedJSON struct {
			Carried bool     `json:"carried"`
			Dropped []string `json:"dropped,omitempty"`
		}
		out := struct {
			From     string       `json:"from"`
			To       string       `json:"to"`
			Tasks    []taskJSON   `json:"tasks"`
			Unlinked []unlinkJSON `json:"unlinked,omitempty"`
			Scoped   *scopedJSON  `json:"scoped,omitempty"`
			Resumed  bool         `json:"resumed,omitempty"`
		}{From: p.From, To: p.To, Resumed: res.Resumed}
		for _, m := range res.Moved {
			out.Tasks = append(out.Tasks, taskJSON{m.ULID.String(), m.OldID, m.NewID})
		}
		for _, u := range p.Unlinked {
			out.Unlinked = append(out.Unlinked, unlinkJSON{u.Task.String(), u.Field, u.Target.String()})
		}
		if p.Scoped != nil {
			out.Scoped = &scopedJSON{Carried: res.Carried, Dropped: p.Scoped.Dropped}
		}
		return a.printJSON(out)
	}
	var b strings.Builder
	if res.Resumed {
		// The destination was written by an earlier run that stopped; this
		// one only finished the source.
		fmt.Fprintf(&b, "Finished the move: %s was left unfinished last time\n", p.From)
	}
	for _, m := range res.Moved {
		fmt.Fprintf(&b, "Moved %s (%s) → %s %s\n", m.OldID, m.ULID, p.To, m.NewID)
	}
	if !shown {
		for _, l := range p.Scoped.Lines(p.To) {
			b.WriteString(l + "\n")
		}
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}
