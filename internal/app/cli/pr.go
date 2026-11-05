package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/integration"
)

func (a *app) prCmd() *cobra.Command {
	var in integration.PRInput
	cmd := &cobra.Command{
		Use:   "pr <id> [--draft]",
		Short: "Push the branch of a task, open its pull request and link it",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := a.getwd()
			if err != nil {
				return err
			}
			in.Dir = dir
			r, err := integration.PullRequest(a.ss, args[0], in)
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
			done := "Pushed " + r.Branch + " of"
			switch {
			case r.Created:
				done = "Opened " + r.Ref.Label() + " for"
			case r.Committed:
				done = "Linked " + r.Ref.Label() + " to"
			}
			if err := a.printWrite(w.TaskWrite, done); err != nil {
				return err
			}
			for _, x := range r.Warnings {
				if _, err := fmt.Fprintf(a.stderr, "warning: %s\n", x); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&in.Draft, "draft", false, "open the pull request as a draft")
	return cmd
}
