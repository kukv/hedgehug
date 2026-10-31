package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
)

func (a *app) inboxCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Show what happened to you: mentions, assignments, overwritten values, renumbered ids",
		Args:  asUsage(cobra.NoArgs),
		RunE:  func(_ *cobra.Command, _ []string) error { return a.inboxList(all) },
	}
	cmd.Flags().BoolVar(&all, "all", false, "show the items you have read too")

	var readAll bool
	read := &cobra.Command{
		Use:   "read <ULID>... | --all",
		Short: "Mark items read, by their full ULIDs",
		Args: func(_ *cobra.Command, args []string) error {
			switch {
			case readAll && len(args) > 0:
				return usageErrorf("give the ULIDs or --all, not both")
			case !readAll && len(args) == 0:
				return usageErrorf("give the ULIDs of the items, or --all")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error { return a.inboxRead(args, readAll) },
	}
	read.Flags().BoolVar(&readAll, "all", false, "mark every item read")

	revert := &cobra.Command{
		Use:   "revert <ULID>",
		Short: "Write your overwritten value back",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.inboxRevert(args[0]) },
	}
	cmd.AddCommand(read, revert)
	return cmd
}

// openInbox opens the stores the command works on. The warnings of the
// opening go to stderr.
func (a *app) openInbox() ([]ops.InboxStore, error) {
	stores, warnings, err := a.ss.InboxStores()
	for _, w := range warnings {
		_, _ = fmt.Fprintln(a.stderr, "hedgehug: warning: "+w)
	}
	return stores, err
}

func (a *app) inboxList(all bool) error {
	stores, err := a.openInbox()
	if err != nil {
		return err
	}
	rows, err := a.ss.InboxListOf(stores, all)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(rows)
	}
	if len(rows) == 0 {
		msg := "No unread items"
		if all {
			msg = "No items"
		}
		_, err := fmt.Fprintln(a.stdout, msg)
		return err
	}
	st := a.style()
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s  %-16s  %s  %s\n", st.id(r.ULID.String()), r.Kind, r.Target, r.Content)
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

func (a *app) inboxRead(args []string, all bool) error {
	ulids, err := ops.ParseInboxItems(args)
	if err != nil {
		return err
	}
	stores, err := a.openInbox()
	if err != nil {
		return err
	}
	read, err := a.ss.InboxReadOf(stores, ulids, all)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(map[string][]string{"read": read})
	}
	_, err = fmt.Fprintf(a.stdout, "Marked %d %s read\n", len(read), plural(len(read), "item", "items"))
	return err
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (a *app) inboxRevert(arg string) error {
	u, err := ops.ParseInboxItem(arg)
	if err != nil {
		return err
	}
	stores, err := a.openInbox()
	if err != nil {
		return err
	}
	res, err := a.ss.InboxRevertOf(stores, u)
	if err != nil {
		return err
	}
	return a.printTask(res.Index, res.ULID, verb(res.Committed, "Reverted"))
}
