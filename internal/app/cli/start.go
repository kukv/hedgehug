package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/integration"
)

func (a *app) startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id>",
		Short: "Start a task: make its branch, assign it to you and move it to doing",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := a.getwd()
			if err != nil {
				return err
			}
			r, err := integration.Start(a.ss, args[0], dir)
			if err != nil {
				return err
			}
			w, err := r.Write()
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(w)
			}
			if err := a.printWrite(w.TaskWrite, verb(r.Committed, "Started")); err != nil {
				return err
			}
			line := "On branch " + r.Branch
			if r.Switch {
				line = "The branch is there; run: git switch " + r.Branch
			}
			_, err = fmt.Fprintln(a.stdout, line)
			return err
		},
	}
}
