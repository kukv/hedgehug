package main

import "github.com/spf13/cobra"

func (a *app) lsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [query]",
		Short: "List tasks",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, _, err := a.open()
			return err
		},
	}
}
