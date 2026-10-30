package main

import "github.com/spf13/cobra"

func (a *app) schemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Work on the schema of the store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.AddCommand(a.migrateCmd())
	return cmd
}
