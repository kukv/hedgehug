package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

func (a *app) scopeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "Show, change or remove the override of the schema for one project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	var statuses string
	var transitions, required []string
	set := &cobra.Command{
		Use:   "set <project>",
		Short: "Change the statuses and the workflow of one project and its descendants",
		Long: "Replace the parts that are given and keep the others. An empty value takes a part off, so the store's value applies again:\n" +
			"  --statuses \"\"  --transition \"\"  --required \"\"",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ch schema.ScopedChange
			f := cmd.Flags()
			if f.Changed("statuses") {
				v, err := splitNames("--statuses", statuses)
				if err != nil {
					return err
				}
				ch.Statuses = &v
			}
			var err error
			if f.Changed("transition") {
				if ch.Transitions, err = listsFlag("--transition", transitions); err != nil {
					return err
				}
			}
			if f.Changed("required") {
				if ch.Required, err = listsFlag("--required", required); err != nil {
					return err
				}
			}
			if ch.Statuses == nil && ch.Transitions == nil && ch.Required == nil {
				return usageErrorf("give --statuses, --transition or --required")
			}
			r, err := a.ss.ScopedSet(args[0], ch)
			if err != nil {
				return err
			}
			return a.printScoped(r, "Changed", "Nothing to change in", true)
		},
	}
	set.Flags().StringVar(&statuses, "statuses", "", "the statuses to show, such as todo,in_progress,done,canceled")
	set.Flags().StringArrayVar(&transitions, "transition", nil, "a status and the statuses it may move to, such as todo=in_progress,done (repeatable)")
	set.Flags().StringArrayVar(&required, "required", nil, "a status and the fields needed to enter it, such as done=points,due (repeatable)")
	cmd.AddCommand(
		&cobra.Command{
			Use:   "show <project>",
			Short: "Show the override of one project",
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				r, err := a.ss.ScopedGet(args[0])
				if err != nil {
					return err
				}
				return a.printScoped(r, "", "", true)
			},
		},
		set,
		&cobra.Command{
			Use:   "rm <project>",
			Short: "Remove the override of one project, templates included",
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				r, err := a.ss.ScopedRemove(args[0])
				if err != nil {
					return err
				}
				return a.printScoped(r, "Removed", "No override to remove from", false)
			},
		},
	)
	return cmd
}

// splitNames splits a comma separated list; an empty value is an empty list.
func splitNames(flag, v string) ([]string, error) {
	out := []string{}
	if v == "" {
		return out, nil
	}
	for _, n := range strings.Split(v, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			return nil, usageErrorf("%s: %q has an empty name", flag, v)
		}
		out = append(out, n)
	}
	return out, nil
}

// listsFlag reads the values of a repeatable flag of the form key=a,b. One
// empty value alone is an empty set of lists, which takes the part off.
func listsFlag(flag string, vals []string) (*schema.Lists, error) {
	l := schema.Lists{}
	if len(vals) == 1 && vals[0] == "" {
		return &l, nil
	}
	for _, v := range vals {
		key, rest, ok := strings.Cut(v, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, usageErrorf("%s: %q is not of the form key=a,b", flag, v)
		}
		if _, dup := l.Get(key); dup {
			return nil, usageErrorf("%s: %q is given twice", flag, key)
		}
		names, err := splitNames(flag, rest)
		if err != nil {
			return nil, err
		}
		l = append(l, schema.List{Key: key, Values: names})
	}
	return &l, nil
}

// printScoped prints the override r is about. A write starts with a line
// that says what it did, done, or that it changed nothing, none; both are
// empty for show. With body the override as it is now follows.
func (a *app) printScoped(r usecase.ScopedResult, done, none string, body bool) error {
	if a.jsonOut {
		return a.printJSON(r.View())
	}
	name := r.Project.String()
	if r.Entry != nil {
		name = r.Index.DisplayID(r.Entry) + "  " + r.Entry.Title
	}
	var b strings.Builder
	if done != "" {
		if !r.Committed {
			_, err := fmt.Fprintf(a.stdout, "%s %s\n", none, name)
			return err
		}
		fmt.Fprintf(&b, "%s the override of %s\n", done, name)
	}
	if body {
		writeScoped(&b, name, r.Scoped)
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}

func writeScoped(b *strings.Builder, name string, sc *schema.Scoped) {
	if sc == nil {
		fmt.Fprintf(b, "%s: no override\n", name)
		return
	}
	fmt.Fprintf(b, "%s\n", name)
	if sc.Statuses != nil {
		fmt.Fprintf(b, "  statuses: %s\n", strings.Join(sc.Statuses, ", "))
	}
	if w := sc.Workflow; w != nil {
		writeLists(b, "transitions", w.Transitions)
		writeLists(b, "required", w.Required)
	}
	if sc.Templates != nil {
		b.WriteString("  templates: kept (they are edited in schema.yaml)\n")
	}
}

func writeLists(b *strings.Builder, heading string, l schema.Lists) {
	if len(l) == 0 {
		return
	}
	fmt.Fprintf(b, "  %s:\n", heading)
	for _, x := range l {
		fmt.Fprintf(b, "    %s: %s\n", x.Key, strings.Join(x.Values, ", "))
	}
}
