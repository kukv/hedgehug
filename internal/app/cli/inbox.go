package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/lib/plural"
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

func (a *app) inboxList(all bool) error {
	stores, err := notifications.InboxStores(a.ss)
	if err != nil {
		return err
	}
	rows, err := notifications.InboxListOf(a.ss, stores, all)
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
	ulids, err := notifications.ParseInboxItems(args)
	if err != nil {
		return err
	}
	stores, err := notifications.InboxStores(a.ss)
	if err != nil {
		return err
	}
	read, err := notifications.InboxReadOf(a.ss, stores, ulids, all)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(map[string][]string{"read": read})
	}
	_, err = fmt.Fprintf(a.stdout, "Marked %s read\n", plural.Count(len(read), "item"))
	return err
}

func (a *app) inboxRevert(arg string) error {
	u, err := notifications.ParseInboxItem(arg)
	if err != nil {
		return err
	}
	stores, err := notifications.InboxStores(a.ss)
	if err != nil {
		return err
	}
	res, err := notifications.InboxRevertOf(a.ss, stores, u)
	if err != nil {
		return err
	}
	return a.printTask(res.Index, res.ULID, verb(res.Committed, "Reverted"))
}
