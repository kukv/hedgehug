package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
)

func (a *app) useCmd() *cobra.Command {
	var doClear bool
	cmd := &cobra.Command{
		Use:   "use [<project>] [--clear]",
		Short: "Set the project new tasks go under, show it, or clear it",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			if doClear && len(args) == 1 {
				return usageErrorf("give a project or --clear, not both")
			}
			switch {
			case doClear:
				return a.useClear()
			case len(args) == 1:
				return a.useSet(args[0])
			}
			return a.useShow()
		},
	}
	cmd.Flags().BoolVar(&doClear, "clear", false, "stop using a project")
	return cmd
}

func (a *app) useSet(arg string) error {
	res, err := a.ss.Use(arg)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(useJSON(res))
	}
	_, err = fmt.Fprintf(a.stdout, "→ %s / %s is the default for new tasks\n", res.Store, res.Project.Title)
	return err
}

func (a *app) useShow() error {
	res, err := a.ss.UseShow()
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(useJSON(res))
	}
	if res.Project == nil {
		_, err = fmt.Fprintln(a.stdout, "-")
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "%s / %s %s\n", res.Store, a.style().id(res.Index.DisplayID(res.Project)), res.Project.Title)
	return err
}

func (a *app) useClear() error {
	res, err := a.ss.UseClear()
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(useJSON(ops.UseResult{}))
	}
	msg := "No changes"
	if res.Changed {
		msg = "Cleared"
	}
	_, err = fmt.Fprintln(a.stdout, msg)
	return err
}

// useProject is the project in the JSON of hh use.
type useProject struct {
	ID    string `json:"id"`
	ULID  string `json:"ulid"`
	Title string `json:"title"`
}

// useJSON is the JSON of hh use: {store, project}, or {} when there is none.
func useJSON(res ops.UseResult) any {
	if res.Project == nil {
		return struct{}{}
	}
	return struct {
		Store   string     `json:"store"`
		Project useProject `json:"project"`
	}{res.Store, useProject{res.Index.DisplayID(res.Project), res.Project.ULID.String(), res.Project.Title}}
}
