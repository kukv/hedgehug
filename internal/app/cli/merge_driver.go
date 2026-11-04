package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/replication"
)

// mergeDriverCmd is the command .git/config names for the files
// .gitattributes sends to the hedgehug driver. git runs it in the work tree,
// in the middle of a merge, so it never starts a background sync.
func (a *app) mergeDriverCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "merge-driver <base> <ours> <theirs> <path>",
		Short:  "The git merge driver of the store's files",
		Hidden: true,
		Args:   exactArgs(4),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			return replication.MergeDriver(a.ss, replication.MergeDriverInput{
				Dir: dir, Base: args[0], Ours: args[1], Theirs: args[2], Path: args[3], Environ: os.Environ(),
			})
		},
	}
}
