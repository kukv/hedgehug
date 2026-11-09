package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/convert"
)

func (a *app) storeConvertCmd() *cobra.Command {
	var dryRun, dropBad bool
	var kinds []string
	cmd := &cobra.Command{
		Use:   "convert [--dry-run] [--kind <old>=<new>]... [--drop-bad-values]",
		Short: "Convert a store in an older format to the newest one",
		Long: "Convert the store to the newest format, one step after the other, and write all of it as one commit.\n" +
			"A store with a remote is converted only when it is in sync with it, and the result is pushed.\n" +
			"--dry-run lists what each step would change and writes nothing.\n" +
			"--kind moves the tasks of a kind to one of the fixed kinds (project, epic, story, task, bug, step): a kind of\n" +
			"your own is otherwise a project when it groups work at the root of a workspace, an epic when it groups work\n" +
			"below, and a task when it is a work item.\n" +
			"--drop-bad-values drops the values a step cannot convert (such as a relation that is not the ULID of a task, or an estimate that is not a duration)\n" +
			"and lists each in the report; without it the conversion stops and lists them.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			opt := convert.Options{Kinds: map[string]string{}, DropBadValues: dropBad}
			for _, k := range kinds {
				from, to, ok := strings.Cut(k, "=")
				if !ok || from == "" || to == "" {
					return usageErrorf("--kind wants <old>=<new>, such as --kind sprint=epic: %q", k)
				}
				opt.Kinds[from] = to
			}
			reports, err := convert.Run(a.ss, opt, dryRun)
			if a.jsonOut && (err == nil || reports != nil) {
				if perr := a.printJSON(convert.ResultOf(reports, dryRun)); perr != nil {
					return perr
				}
				return err
			}
			if len(reports) == 0 && err == nil {
				_, err = fmt.Fprintln(a.stdout, "The store is already in the newest format.")
				return err
			}
			for _, r := range reports {
				_, _ = fmt.Fprintf(a.stdout, "Format %d -> %d\n", r.From, r.To)
				for _, l := range r.Lines {
					_, _ = fmt.Fprintf(a.stdout, "  %s\n", l)
				}
			}
			if err == nil && dryRun {
				_, err = fmt.Fprintln(a.stdout, "Nothing was written (--dry-run).")
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list what would change, and write nothing")
	cmd.Flags().BoolVar(&dropBad, "drop-bad-values", false, "drop the values that cannot be converted, and list them, instead of stopping")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "move the tasks of a kind to a fixed kind, such as sprint=epic (repeatable)")
	return cmd
}
