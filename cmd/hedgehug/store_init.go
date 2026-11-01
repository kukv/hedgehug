package main

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
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
	cmd.AddCommand(a.storeInitCmd(), a.storeAddCmd(), a.storeListCmd(), a.storeRemoveCmd(), a.storeDefaultCmd())
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
			switch template {
			case "":
			case "dev":
				sc = schema.Dev()
			default:
				return usageErrorf("--template: %q is not a template; the one there is is dev", template)
			}
			return a.storeInit(ops.StoreInitInput{Name: args[0], Prefix: prefix, Workspace: wsName, Schema: sc, Remotes: remotes, Account: account, Path: path})
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "the id prefix of the first workspace, such as WEB")
	cmd.Flags().StringVar(&wsName, "workspace", "", "the name of the first workspace (default: the store's name)")
	cmd.Flags().StringArrayVar(&remotes, "remote", nil, "a remote to push the store to, which must be empty; the first is origin, the others are mirrors")
	cmd.Flags().StringVar(&account, "account", "", "the account that writes the store and makes its GitHub repository (default: the only account)")
	cmd.Flags().StringVar(&path, "path", "", "where to make the store (default: under the data directory)")
	cmd.Flags().StringVar(&template, "template", "", "start from a template: dev adds a review status")
	return cmd
}

func (a *app) storeInit(in ops.StoreInitInput) error {
	r, err := a.ss.StoreInit(in)
	// A remote on GitHub with no account to make it: on a terminal, ask which.
	if e := ops.AsError(err); e != nil && e.Code == "choice.required" && e.Args["what"] == "--account" && a.isTerminal(a.stdin) {
		candidates, _ := e.Args["candidates"].([]string)
		var picked string
		if picked, err = a.askChoice("Which account makes the repository?", candidates); err != nil {
			return err
		}
		in.Account = picked
		r, err = a.ss.StoreInit(in)
	}
	if err != nil {
		return err
	}
	ws := r.Workspace
	if a.jsonOut {
		type wsJSON struct {
			ULID     string `json:"ulid"`
			Name     string `json:"name"`
			IDPrefix string `json:"id_prefix"`
		}
		return a.printJSON(struct {
			Name      string   `json:"name"`
			Path      string   `json:"path"`
			Workspace wsJSON   `json:"workspace"`
			Remotes   []string `json:"remotes,omitempty"`
			Pushed    []string `json:"pushed,omitempty"`
		}{r.Name, r.Path, wsJSON{ws.ULID.String(), ws.Name, ws.IDPrefix}, r.Remotes, r.Pushed})
	}
	_, err = fmt.Fprintf(a.stdout, "Created store %s at %s with workspace %s (%s)\n", r.Name, r.Path, ws.Name, ws.IDPrefix)
	if err == nil && len(r.Pushed) > 0 {
		_, err = fmt.Fprintf(a.stdout, "Pushed to %s\n", strings.Join(r.Pushed, ", "))
	}
	return err
}

// askChoice lists candidates on stderr and reads the number of the one to
// pick from stdin.
func (a *app) askChoice(question string, candidates []string) (string, error) {
	var b strings.Builder
	b.WriteString(question + "\n")
	for i, c := range candidates {
		fmt.Fprintf(&b, "  %d) %s\n", i+1, c)
	}
	b.WriteString("Number: ")
	if _, err := fmt.Fprint(a.stderr, b.String()); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(a.stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("canceled")
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(candidates) {
		return "", usageErrorf("%q is not one of 1-%d", strings.TrimSpace(line), len(candidates))
	}
	return candidates[n-1], nil
}
