package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (a *app) storeAddCmd() *cobra.Command {
	var remotes []string
	var path, account string
	cmd := &cobra.Command{
		Use:   "add <name> --remote <url>...",
		Short: "Add a store that others made, by cloning it",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(remotes) == 0 {
				return usageErrorf("--remote is required: the remote to clone the store from")
			}
			return a.storeAdd(args[0], remotes, path, account)
		},
	}
	cmd.Flags().StringArrayVar(&remotes, "remote", nil, "a remote of the store; the first is cloned, the others are mirrors")
	cmd.Flags().StringVar(&path, "path", "", "where to clone the store (default: under the data directory, as store init does)")
	cmd.Flags().StringVar(&account, "account", "", "the account that reaches the store (default: the only account)")
	return cmd
}

func (a *app) storeAdd(name string, remotes []string, path, account string) error {
	r, err := a.ss.StoreAdd(name, remotes, path, account)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(r)
	}
	_, err = fmt.Fprintf(a.stdout, "Added store %s at %s from %s\n", r.Name, r.Path, r.Remotes[0])
	return err
}

func (a *app) storeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the stores that are set up",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.storeList() },
	}
}

func (a *app) storeList() error {
	out, err := a.ss.StoreList()
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(out)
	}
	if len(out) == 0 {
		_, err := fmt.Fprintln(a.stdout, "No store is set up")
		return err
	}
	rows := make([][]string, len(out))
	for i, x := range out {
		remotes, account := strings.Join(x.Remotes, ","), x.Account
		if remotes == "" {
			remotes = "-"
		}
		if account == "" {
			account = "-"
		}
		rows[i] = []string{x.Name, x.Path, remotes, account, timeText(x.LastSuccess)}
	}
	return writeTable(a.stdout, []string{"NAME", "PATH", "REMOTES", "ACCOUNT", "LAST SYNC"}, rows, nil)
}

func (a *app) storeRemoveCmd() *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Take a store out of the config",
		Long:  "Remove takes the store out of config.yaml and leaves its directory. With --purge it deletes the directory too, unless commits are not pushed yet.",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.storeRemove(args[0], purge) },
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "delete the directory of the store too")
	return cmd
}

func (a *app) storeRemove(name string, purge bool) error {
	r, err := a.ss.StoreRemove(name, purge)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(r)
	}
	if purge {
		_, err = fmt.Fprintf(a.stdout, "Removed store %s and deleted %s\n", name, r.Path)
	} else {
		_, err = fmt.Fprintf(a.stdout, "Removed store %s from the config; %s is left\n", name, r.Path)
	}
	return err
}
