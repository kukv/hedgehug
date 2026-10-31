package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (a *app) storeDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default [<name>]",
		Short: "Show or set the default store, where hedgehug add puts a task when nothing else decides",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			c, _, err := a.config()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				if err := c.SetDefaultStore(args[0]); err != nil {
					names := make([]string, len(c.Stores))
					for i, s := range c.Stores {
						names[i] = s.Name
					}
					return fmt.Errorf("%w (stores: %s)", err, strings.Join(names, ", "))
				}
				if err := c.Save(); err != nil {
					return err
				}
			}
			if a.jsonOut {
				var d *string
				if c.DefaultStore != "" {
					d = &c.DefaultStore
				}
				return a.printJSON(struct {
					Default *string `json:"default"`
				}{d})
			}
			name := c.DefaultStore
			if name == "" {
				name = "-"
			}
			_, err = fmt.Fprintln(a.stdout, name)
			return err
		},
	}
}
