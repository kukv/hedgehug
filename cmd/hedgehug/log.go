package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/ops"
)

func (a *app) logCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "log [id]",
		Short: "Show the commits of the store, or of one task",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			if limit < 0 {
				return usageErrorf("-n must not be negative")
			}
			return a.log(args, limit)
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "show at most this many commits (0 for all)")
	return cmd
}

func logText(ix *index.Index, entries []gitrepo.LogEntry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s  %s  %s  %s\n", e.Hash.String()[:7], e.When.In(time.Local).Format("2006-01-02 15:04"), e.Author.Name, ops.SubjectNow(ix, e))
	}
	return b.String()
}

func (a *app) log(args []string, limit int) error {
	ix, entries, err := a.ss.Log(args, limit)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(ops.LogViewsOf(ix, entries))
	}
	_, err = fmt.Fprint(a.stdout, logText(ix, entries))
	return err
}
