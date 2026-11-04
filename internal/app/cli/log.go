package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
)

func (a *app) logCmd() *cobra.Command {
	var limit int
	var workspaces []string
	cmd := &cobra.Command{
		Use:   "log [id]",
		Short: "Show the commits of the store, or of one task",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			if limit < 0 {
				return usageErrorf("-n must not be negative")
			}
			return a.log(args, workspaces, limit)
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "show at most this many commits (0 for all)")
	cmd.Flags().StringSliceVar(&workspaces, "workspace", nil, "only the commits of these workspaces (name, prefix or ULID; comma-separated)")
	return cmd
}

func logLine(l usecase.LogView) string {
	return fmt.Sprintf("%s  %s  %s  %s\n", l.Hash[:7], l.Date.In(time.Local).Format("2006-01-02 15:04"), l.Author.Name, l.CurrentSubject)
}

func logText(logs []usecase.LogView) string {
	var b strings.Builder
	for _, l := range logs {
		b.WriteString(logLine(l))
	}
	return b.String()
}

// storeLogText is the commits of logs, each led by the name of its store when
// many says there is more than one, padded to the longest.
func storeLogText(logs []usecase.LogView, many bool) string {
	width := 0
	if many {
		for _, l := range logs {
			width = max(width, displayWidth(l.Store))
		}
	}
	var b strings.Builder
	for _, l := range logs {
		if many {
			b.WriteString(l.Store + strings.Repeat(" ", width-displayWidth(l.Store)) + "  ")
		}
		b.WriteString(logLine(l))
	}
	return b.String()
}

func (a *app) log(args, workspaces []string, limit int) error {
	res, err := browse.LogIn(a.ss, args, workspaces, limit)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(res.Logs)
	}
	_, err = fmt.Fprint(a.stdout, storeLogText(res.Logs, res.Many))
	return err
}
