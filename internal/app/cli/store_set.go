package cli

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/spf13/cobra"
)

func (a *app) storeSetCmd() *cobra.Command {
	var color string
	var hidden bool
	cmd := &cobra.Command{
		Use:   "set <name> [--color <color>] [--hidden=true|false]",
		Short: "Set the color of a store and whether the GUI shows it, on this machine only",
		Long: "Set how the GUI shows a store. Only what is given changes; --color \"\" gives the color back to the GUI to pick.\n" +
			"A hidden store is left out of the GUI's lists; the CLI still works on it.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var c *string
			var h *bool
			if cmd.Flags().Changed("color") {
				c = &color
			}
			if cmd.Flags().Changed("hidden") {
				h = &hidden
			}
			if c == nil && h == nil {
				return usageErrorf("--color or --hidden is required")
			}
			v, err := stores.StoreSet(a.ss, args[0], c, h)
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(v)
			}
			shown := v.Color
			if shown == "" {
				shown = "-"
			}
			_, err = fmt.Fprintf(a.stdout, "Set %s: color=%s hidden=%t\n", v.Name, shown, v.Hidden)
			return err
		},
	}
	cmd.Flags().StringVar(&color, "color", "", "teal, orange, purple, sky, pink, olive or brown; \"\" gives the color back to the GUI")
	cmd.Flags().BoolVar(&hidden, "hidden", false, "leave the store out of the GUI's lists (--hidden=false shows it again)")
	return cmd
}
