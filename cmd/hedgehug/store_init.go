package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
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
	cmd.AddCommand(a.storeInitCmd())
	return cmd
}

func (a *app) storeInitCmd() *cobra.Command {
	var template, prefix, wsName string
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
			return a.storeInit(args[0], prefix, wsName, sc)
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "the id prefix of the first workspace, such as WEB")
	cmd.Flags().StringVar(&wsName, "workspace", "", "the name of the first workspace (default: the store's name)")
	cmd.Flags().StringVar(&template, "template", "", "start from a template: dev adds a review status")
	return cmd
}

func (a *app) storeInit(name, prefix, wsName string, sc *schema.Schema) error {
	if err := config.ValidStoreName(name); err != nil {
		return usageError{err}
	}
	if err := model.ValidIDPrefix(prefix); err != nil {
		return usageError{err}
	}
	if wsName == "" {
		wsName = name
	}
	if err := model.ValidWorkspaceName(wsName); err != nil {
		return usageError{err}
	}
	c, p, err := a.config()
	if err != nil {
		return err
	}
	for _, e := range c.Stores {
		if e.Name == name {
			return fmt.Errorf("a store named %q is already set up", name)
		}
	}
	if err := a.checkPrefixElsewhere(prefix, name); err != nil {
		return err
	}
	author, err := config.Author(a.getenv)
	if err != nil {
		return err
	}
	dir := filepath.Join(p.StoresDir, name)
	ws := &model.Workspace{ULID: model.NewULID(), Name: wsName, IDPrefix: prefix}
	if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, sc, ws, author); err != nil {
		return err
	}
	if err := c.AddStore(name, dir); err != nil {
		return errors.Join(err, os.RemoveAll(dir))
	}
	if err := c.Save(); err != nil {
		return errors.Join(err, os.RemoveAll(dir))
	}
	if a.jsonOut {
		type wsJSON struct {
			ULID     string `json:"ulid"`
			Name     string `json:"name"`
			IDPrefix string `json:"id_prefix"`
		}
		return a.printJSON(struct {
			Name      string `json:"name"`
			Path      string `json:"path"`
			Workspace wsJSON `json:"workspace"`
		}{name, dir, wsJSON{ws.ULID.String(), ws.Name, ws.IDPrefix}})
	}
	_, err = fmt.Fprintf(a.stdout, "Created store %s at %s with workspace %s (%s)\n", name, dir, ws.Name, ws.IDPrefix)
	return err
}
