package cli

import (
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/spf13/cobra"
)

func (a *app) depCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage the relations between tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	for _, add := range []bool{true, false} {
		flags := map[string]*[]string{}
		name, short := "add", "Tie a task to other tasks through relations"
		if !add {
			name, short = "rm", "Untie a task from other tasks"
		}
		sub := &cobra.Command{
			Use:   name + " <id> " + depFlagsUsage(),
			Short: short,
			Long:  short + ".\n\nEach flag takes tasks, and can be given again; all of them are written in one commit.",
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				targets := tasks.DepTargets{}
				for rel, v := range flags {
					if len(*v) > 0 {
						targets[rel] = *v
					}
				}
				if len(targets) == 0 {
					return usageErrorf("give at least one of %s", depFlagNames())
				}
				return a.dep(args[0], targets, add)
			},
		}
		for _, rel := range schema.Relations {
			var v []string
			flags[rel.Name] = &v
			sub.Flags().StringSliceVar(&v, strings.ReplaceAll(rel.Name, "_", "-"), nil, "the tasks "+strings.ToLower(rel.Label)+" (a task, comma separated or repeated)")
		}
		cmd.AddCommand(sub)
	}
	return cmd
}

// depFlagsUsage is the flags of dep add and dep rm as the usage line shows them.
func depFlagsUsage() string {
	var parts []string
	for _, rel := range schema.Relations {
		parts = append(parts, "[--"+strings.ReplaceAll(rel.Name, "_", "-")+" <id>...]")
	}
	return strings.Join(parts, " ")
}

func depFlagNames() string {
	var names []string
	for _, rel := range schema.Relations {
		names = append(names, "--"+strings.ReplaceAll(rel.Name, "_", "-"))
	}
	return strings.Join(names, ", ")
}

func (a *app) dep(arg string, targets tasks.DepTargets, add bool) error {
	r, err := tasks.Dep(a.ss, arg, targets, add)
	if err != nil {
		return err
	}
	return a.printTask(r, verb(r.Committed, "Updated"))
}
