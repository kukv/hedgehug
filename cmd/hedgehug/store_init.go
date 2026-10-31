package main

import (
	"fmt"
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
	cmd.AddCommand(a.storeInitCmd(), a.storeAddCmd(), a.storeListCmd(), a.storeRemoveCmd())
	return cmd
}

func (a *app) storeInitCmd() *cobra.Command {
	var template, prefix, wsName string
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
			return a.storeInit(args[0], prefix, wsName, sc, remotes)
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "the id prefix of the first workspace, such as WEB")
	cmd.Flags().StringVar(&wsName, "workspace", "", "the name of the first workspace (default: the store's name)")
	cmd.Flags().StringArrayVar(&remotes, "remote", nil, "a remote to push the store to, which must be empty; the first is origin, the others are mirrors")
	cmd.Flags().StringVar(&template, "template", "", "start from a template: dev adds a review status")
	return cmd
}

func (a *app) storeInit(name, prefix, wsName string, sc *schema.Schema, remotes []string) error {
	r, err := a.ss.StoreInit(ops.StoreInitInput{Name: name, Prefix: prefix, Workspace: wsName, Schema: sc, Remotes: remotes})
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
