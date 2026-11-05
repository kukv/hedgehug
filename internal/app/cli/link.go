package cli

import (
	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/integration"
)

func (a *app) linkCmd() *cobra.Command {
	var in integration.LinkInput
	cmd := &cobra.Command{
		Use:   "link <id> (--pr owner/repo#n | --issue owner/repo#n) [--sync mirror|push|link]",
		Short: "Link a task to a pull request or an issue",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			r, err := integration.Link(a.ss, args[0], in)
			if err != nil {
				return err
			}
			return a.printTask(r, verb(r.Committed, "Linked"))
		},
	}
	linkFlags(cmd, &in)
	cmd.Flags().StringVar(&in.Sync, "sync", "", "how an issue syncs: mirror, push or link (default link)")
	return cmd
}

func (a *app) unlinkCmd() *cobra.Command {
	var in integration.LinkInput
	cmd := &cobra.Command{
		Use:   "unlink <id> (--pr owner/repo#n | --issue owner/repo#n)",
		Short: "Unlink a task from a pull request or an issue",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			r, err := integration.Unlink(a.ss, args[0], in)
			if err != nil {
				return err
			}
			return a.printTask(r, verb(r.Committed, "Unlinked"))
		},
	}
	linkFlags(cmd, &in)
	return cmd
}

func linkFlags(cmd *cobra.Command, in *integration.LinkInput) {
	cmd.Flags().StringVar(&in.PR, "pr", "", "the pull request, as owner/repo#n")
	cmd.Flags().StringVar(&in.Issue, "issue", "", "the issue, as owner/repo#n")
}
