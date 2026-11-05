package cli

import (
	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

func (a *app) setCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <id> <key=value>...",
		Short: "Change fields and properties of a task",
		Long: `Change fields and properties of a task.

  key=value   set (a list becomes just value)
  key=        clear
  key+=value  add to a list
  key-=value  remove from a list

Fields: title, kind, status, assignees, watchers, labels, repos, blocked_by,
sprint, releases (a name in the workspace of the task).
Any other key is a property of the schema. Use hedgehug mv for the parent
and the order.`,
		Args: minArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			assigns, err := usecase.ParseAssigns(args[1:])
			if err != nil {
				return err
			}
			return a.set(args[0], assigns)
		},
	}
}

func (a *app) set(arg string, assigns []usecase.Assign) error {
	r, err := tasks.Set(a.ss, arg, assigns)
	if err != nil {
		return err
	}
	return a.printTask(r, verb(r.Committed, "Updated"))
}
