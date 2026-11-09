package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) warnAll(warnings []string) error {
	for _, x := range warnings {
		if _, err := fmt.Fprintf(a.stderr, "warning: %s\n", x); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) issueCmd() *cobra.Command {
	var in integration.IssueInput
	cmd := &cobra.Command{
		Use:   "issue <id> [--repo org/repo]",
		Short: "Open an issue for a task and link it in push mode",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if in.Repo == "" {
				// Only a task with no repository asks for it.
				if dir, err := a.workdir(); err == nil {
					in.Dir = dir
				}
			}
			r, err := integration.CreateIssue(a.ss, args[0], in)
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
			if err := a.printWrite(w.TaskWrite, "Opened "+r.Ref.Label()+" for"); err != nil {
				return err
			}
			return a.warnAll(r.Warnings)
		},
	}
	cmd.Flags().StringVar(&in.Repo, "repo", "", "the repository, as org/repo (default the task's)")
	return cmd
}

func (a *app) ghCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh",
		Short: "Work with GitHub in bulk",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	var in integration.ImportInput
	imp := &cobra.Command{
		Use:   "import <org/repo> [--label <label>] [--parent <id>] [--workspace <name>] [--dry-run]",
		Short: "Make a task in mirror mode for each open issue (with --label, only those with the label)",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			in.Repo = args[0]
			r, err := integration.ImportIssues(a.ss, in)
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(r)
			}
			issues := plural.Count(r.Count, "issue")
			switch {
			case r.DryRun:
				_, err = fmt.Fprintf(a.stdout, "Would import %s of %s (%d linked already)\n", issues, r.Repo, r.Skipped)
			case r.Count == 0:
				_, err = fmt.Fprintf(a.stdout, "Nothing to import from %s (%d linked already)\n", r.Repo, r.Skipped)
			default:
				_, err = fmt.Fprintf(a.stdout, "Imported %s of %s (%d linked already): %s\n", issues, r.Repo, r.Skipped, strings.Join(r.Created, ", "))
			}
			if err != nil {
				return err
			}
			return a.warnAll(r.Warnings)
		},
	}
	imp.Flags().StringVar(&in.Label, "label", "", "take only the open issues with this label (default: every open issue)")
	imp.Flags().StringVar(&in.Parent, "parent", "", "put the tasks under this task (default: the project of hedgehug use)")
	imp.Flags().StringVar(&in.Workspace, "workspace", "", "put the tasks in this workspace")
	imp.Flags().BoolVar(&in.DryRun, "dry-run", false, "only say how many it would make")
	cmd.AddCommand(imp)
	return cmd
}
