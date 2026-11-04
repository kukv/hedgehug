package cli

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/usecase/stores"
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
	r, err := stores.StoreAdd(a.ss, name, remotes, path, account)
	if err != nil {
		return err
	}
	if a.jsonOut {
		v, err := stores.ViewOf(a.ss, r.Name)
		if err != nil {
			return err
		}
		return a.printJSON(v)
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
	out, err := stores.StoreItems(a.ss)
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
	var keep bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a store and delete its directory",
		Long: `Remove takes the store out of config.yaml and deletes its directory.

It refuses when the store has commits that are not pushed, changes that are not committed, or no remote: the tasks could not be got back. Run hedgehug sync --store <name> first. With --keep it only takes the store out of config.yaml and leaves the directory, and checks nothing.`,
		Args: exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error { return a.storeRemove(args[0], keep) },
	}
	cmd.Flags().BoolVar(&keep, "keep", false, "take the store out of the config and leave its directory")
	return cmd
}

func (a *app) storeRemove(name string, keep bool) error {
	r, err := stores.StoreRemove(a.ss, name, keep)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(r)
	}
	switch {
	case r.Deleted:
		_, err = fmt.Fprintf(a.stdout, "Removed store %s and deleted %s\n", name, r.Path)
	case keep:
		_, err = fmt.Fprintf(a.stdout, "Removed store %s from the config; %s is left\n", name, r.Path)
	default:
		_, err = fmt.Fprintf(a.stdout, "Removed store %s from the config; %s was not there\n", name, r.Path)
	}
	return err
}
