package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
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
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	sc := s.Schema()
	next, err := sc.Migrate(m)
	if err != nil {
		return err
	}
	if ps := next.Check(); len(ps) > 0 {
		return &store.SchemaError{Problems: ps}
	}
	var changed []*model.Task
	var skipped []model.ID
	var refused []string
	for t, err := range s.Tasks() {
		if err != nil {
			id, ok := brokenTask(err)
			if !ok {
				return err
			}
			skipped = append(skipped, id)
			continue
		}
		ok, err := m.ApplyTask(t, sc)
		switch {
		case err != nil:
			refused = append(refused, refName(ix, t.ID)+": "+err.Error())
		case ok:
			changed = append(changed, t)
		}
	}
	if len(refused) > 0 {
		slices.Sort(refused)
		return fmt.Errorf("cannot migrate %s:\n  %s", count(len(refused)), strings.Join(refused, "\n  "))
	}
	slices.SortFunc(changed, func(x, y *model.Task) int { return strings.Compare(x.ID.String(), y.ID.String()) })
	slices.SortFunc(skipped, func(x, y model.ID) int { return strings.Compare(x.String(), y.String()) })

	if !dryRun {
		if _, err := a.update(s, store.Op{Name: "migrate", SkipTransitions: true}, func(tx *store.Tx) error {
			for _, t := range changed {
				if err := tx.PutTask(t); err != nil {
					return err
				}
			}
			return tx.PutSchema(next)
		}); err != nil {
			// Name the task: the migration writes many.
			var ve *store.ValidationError
			if errors.As(err, &ve) {
				return fmt.Errorf("%s: %v", refName(ix, ve.Task), describe(err))
			}
			return err
		}
		if err := ix.Refresh(); err != nil {
			return err
		}
	}
	return a.printMigrated(ix, changed, skipped, dryRun)
}

// brokenTask returns the ID of the task file that err, from Store.Tasks,
// says does not decode. The error starts with the path of the file.
func brokenTask(err error) (model.ID, bool) {
	if !errors.As(err, new(*model.DecodeError)) {
		return model.ID{}, false
	}
	rel, _, _ := strings.Cut(err.Error(), ": ")
	id, perr := model.ParseTaskPath(rel)
	return id, perr == nil
}

func count(n int) string {
	if n == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", n)
}

func (a *app) printMigrated(ix *index.Index, changed []*model.Task, skipped []model.ID, dryRun bool) error {
	shorts := make([]string, len(skipped))
	for i, id := range skipped {
		shorts[i] = ix.ShortID(id)
	}
	if len(skipped) > 0 {
		if _, err := fmt.Fprintf(a.stderr, "warning: not migrated, since they do not decode: %s\n", strings.Join(shorts, ", ")); err != nil {
			return err
		}
	}
	if a.jsonOut {
		out := struct {
			Changed []*taskJSON `json:"changed"`
			Skipped []string    `json:"skipped"`
		}{Changed: []*taskJSON{}, Skipped: shorts}
		for _, t := range changed {
			if e, ok := ix.Task(t.ID); ok {
				out.Changed = append(out.Changed, entryJSON(ix, e))
			}
		}
		return a.printJSON(out)
	}
	heading := "Migrated"
	if dryRun {
		heading = "Would migrate"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", heading, count(len(changed)))
	st := a.style()
	for _, t := range changed {
		fmt.Fprintf(&b, "  %s  %s\n", st.id(ix.ShortID(t.ID)), t.Title)
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}
