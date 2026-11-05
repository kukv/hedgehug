package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/cli/prompt"
	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
)

func (a *app) storeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store",
		Short: "Manage stores",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.AddCommand(a.storeInitCmd(), a.storeAddCmd(), a.storeListCmd(), a.storeRemoveCmd(), a.storeDefaultCmd(), a.storeDiscoverCmd(), a.storeSetCmd())
	return cmd
}

func (a *app) storeInitCmd() *cobra.Command {
	var template, prefix, wsName, account, path string
	var remotes []string
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a store",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if prefix == "" {
				return usageErrorf("--prefix is required: the id prefix of the first workspace, such as WEB")
			}
			var sc *schema.Schema
			var rules *automation.File
			switch template {
			case "":
			case "dev":
				sc, rules = schema.Dev(), automation.DevRules()
			default:
				return usageErrorf("--template: %q is not a template; the one there is is dev", template)
			}
			return a.storeInit(stores.StoreInitInput{Name: args[0], Prefix: prefix, Workspace: wsName, Schema: sc, Automations: rules, Remotes: remotes, Account: account, Path: path})
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "the id prefix of the first workspace, such as WEB")
	cmd.Flags().StringVar(&wsName, "workspace", "", "the name of the first workspace (default: the store's name)")
	cmd.Flags().StringArrayVar(&remotes, "remote", nil, "a remote to push the store to, which must be empty; the first is origin, the others are mirrors")
	cmd.Flags().StringVar(&account, "account", "", "the account that writes the store and makes its GitHub repository (default: the only account)")
	cmd.Flags().StringVar(&path, "path", "", "where to make the store (default: under the data directory)")
	cmd.Flags().StringVar(&template, "template", "", "start from a template: dev adds a review status and the rules that move a task with its pull request")
	return cmd
}

func (a *app) storeInit(in stores.StoreInitInput) error {
	r, err := stores.StoreInit(a.ss, in)
	// A remote on GitHub with no account to make it: on a terminal, ask which.
	if e := usecase.AsError(err); e != nil && e.Code == "choice.required" && e.Args["what"] == "--account" && a.ask().Interactive() {
		candidates, _ := e.Args["candidates"].([]string)
		var picked string
		if picked, err = a.ask().Choose(prompt.Choose{Question: "Which account makes the repository?", Options: candidates, Canceled: "canceled"}); err != nil {
			return err
		}
		in.Account = picked
		r, err = stores.StoreInit(a.ss, in)
	}
	if err != nil {
		return err
	}
	ws := r.Workspace
	if a.jsonOut {
		return a.printJSON(stores.StoreInitViewOf(a.ss.ForStore(r.Name), r))
	}
	_, err = fmt.Fprintf(a.stdout, "Created store %s at %s with workspace %s (%s)\n", r.Name, r.Path, ws.Name, ws.IDPrefix)
	if err == nil && len(r.Pushed) > 0 {
		_, err = fmt.Fprintf(a.stdout, "Pushed to %s\n", strings.Join(r.Pushed, ", "))
	}
	return err
}
