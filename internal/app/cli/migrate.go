package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
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
		{schema.TargetProp, "prop <old> <new>", "Rename a property"},
		{schema.TargetPriority, "priority <old> <new>", "Rename or merge a priority"},
		{schema.TargetLabel, "label <old> <new>", "Rename or merge a label"},
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
	w, err := schemas.Migrate(a.ss, m, dryRun)
	if err != nil {
		return err
	}
	return a.printMigrated(w, dryRun)
}

// printMigrated prints the tasks a migration changes. Their JSON is read
// from the store after a write, and from the migrated tasks in a dry run.
func (a *app) printMigrated(w schemas.MigrateWrite, dryRun bool) error {
	if len(w.Skipped) > 0 {
		if _, err := fmt.Fprintf(a.stderr, "warning: not migrated, since they do not decode: %s\n", strings.Join(w.Skipped, ", ")); err != nil {
			return err
		}
	}
	if a.jsonOut {
		return a.printJSON(w)
	}
	heading := "Migrated"
	if dryRun {
		heading = "Would migrate"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", heading, schemas.TaskCount(len(w.Changed)))
	st := a.style()
	for _, v := range w.Changed {
		fmt.Fprintf(&b, "  %s  %s\n", st.id(v.Label()), v.Title)
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}
