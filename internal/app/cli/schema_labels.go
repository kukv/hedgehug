package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

// labelsCmd is hedgehug schema labels: the labels a task can have are the ones
// the schema lists.
func (a *app) labelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "labels",
		Short: "Add labels to the list of the schema, or remove those no task uses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	for _, x := range []struct {
		use, short, done string
		edit             func(*usecase.Session, []string) (schemas.EditWrite, error)
	}{
		{"add <name>...", "Add labels at the end of the list", "Added", schemas.AddLabels},
		{"rm <name>...", "Remove labels no task uses (those of deleted tasks count)", "Removed", schemas.RemoveLabels},
	} {
		cmd.AddCommand(&cobra.Command{
			Use:   x.use,
			Short: x.short,
			Args:  minArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				cur, err := schemas.SchemaGet(a.ss)
				if err != nil {
					return err
				}
				w, err := x.edit(a.ss, args)
				if err != nil {
					return err
				}
				// the names as the list spelled them before the edit
				names := make([]string, len(args))
				for i, n := range args {
					names[i] = n
					if v, ok := schema.MatchValue(cur.Labels, n); ok {
						names[i] = v
					}
				}
				if a.jsonOut {
					return a.printJSON(w)
				}
				_, err = fmt.Fprintf(a.stdout, "%s %s: %s\n", x.done, plural.Count(len(names), "label"), strings.Join(names, ", "))
				return err
			},
		})
	}
	return cmd
}
