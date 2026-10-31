package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
)

func (a *app) migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Rename a definition of the schema, or merge it into another, and rewrite the tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a target is needed")
		},
	}
	var dryRun bool
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "list the tasks it would rewrite and write nothing")
	for _, x := range []struct {
		target schema.Target
		use    string
		short  string
	}{
		{schema.TargetStatus, "status <old> <new>", "Rename or merge a status"},
		{schema.TargetKind, "kind <old> <new>", "Rename or merge a kind"},
		{schema.TargetProp, "prop <old> <new>", "Rename or merge a property"},
		{schema.TargetValue, "value <prop> <old> <new>", "Rename or merge a value of an enum or multi_enum property"},
	} {
		n := 2
		if x.target == schema.TargetValue {
			n = 3
		}
		cmd.AddCommand(&cobra.Command{
			Use:   x.use,
			Short: x.short,
			Args:  exactArgs(n),
			RunE: func(_ *cobra.Command, args []string) error {
				m := schema.Migration{Target: x.target}
				if x.target == schema.TargetValue {
					m.Prop, args = args[0], args[1:]
				}
				m.Old, m.New = args[0], args[1]
				return a.migrate(m, dryRun)
			},
		})
	}
	return cmd
}

func (a *app) migrate(m schema.Migration, dryRun bool) error {
	ix, p, err := a.ss.Migrate(m, dryRun)
	if err != nil {
		return err
	}
	return a.printMigrated(ix, p, dryRun)
}

// printMigrated prints the tasks p changes. Their JSON is read from the
// index after a write, and from the migrated tasks in a dry run.
func (a *app) printMigrated(ix *index.Index, p *ops.MigrationPlan, dryRun bool) error {
	changed := p.Changed
	skipped := make([]string, len(p.Skipped))
	for i, ulid := range p.Skipped {
		skipped[i] = ulid.String()
	}
	if len(skipped) > 0 {
		if _, err := fmt.Fprintf(a.stderr, "warning: not migrated, since they do not decode: %s\n", strings.Join(skipped, ", ")); err != nil {
			return err
		}
	}
	if a.jsonOut {
		out := struct {
			Changed []*ops.TaskView `json:"changed"`
			Skipped []string        `json:"skipped"`
		}{Changed: []*ops.TaskView{}, Skipped: skipped}
		for _, t := range changed {
			e, ok := ix.Task(t.ULID)
			if dryRun || !ok {
				e = index.EntryOf(t)
			}
			out.Changed = append(out.Changed, ops.TaskViewOf(ix, e))
		}
		return a.printJSON(out)
	}
	heading := "Migrated"
	if dryRun {
		heading = "Would migrate"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", heading, ops.TaskCount(len(changed)))
	st := a.style()
	for _, t := range changed {
		fmt.Fprintf(&b, "  %s  %s\n", st.id(ops.TaskLabel(index.EntryOf(t))), t.Title)
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}
