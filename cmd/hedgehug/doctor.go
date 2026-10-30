package main

import (
	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/doctor"
)

func (a *app) doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Find what is wrong with the files, the work tree and git of the store",
		Args:  exactArgs(0),
		RunE: func(*cobra.Command, []string) error {
			c, _, err := a.config()
			if err != nil {
				return err
			}
			e, err := c.Select(a.storeName)
			if err != nil {
				return err
			}
			fs, err := doctor.Run(e.Path)
			if err != nil {
				return err
			}
			return a.printFindings(fs)
		},
	}
}
