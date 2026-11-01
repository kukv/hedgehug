package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *app) storeDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default [<name>]",
		Short: "Show or set the default store, where hedgehug add puts a task when nothing else decides",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			def, err := a.ss.StoreDefault(name)
			if err != nil {
				return err
			}
			if a.jsonOut {
				var d *string
				if def != "" {
					d = &def
				}
				return a.printJSON(struct {
					Default *string `json:"default"`
				}{d})
			}
			if def == "" {
				def = "-"
			}
			_, err = fmt.Fprintln(a.stdout, def)
			return err
		},
	}
}
