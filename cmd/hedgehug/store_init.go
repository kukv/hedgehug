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
	var template string
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a store",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var sc *schema.Schema
			switch template {
			case "":
			case "dev":
				sc = schema.Dev()
			default:
				return usageErrorf("--template: %q is not a template; the one there is is dev", template)
			}
			return a.storeInit(args[0], sc)
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "start from a template: dev adds a review status")
	return cmd
}

func (a *app) storeInit(name string, sc *schema.Schema) error {
	if err := config.ValidStoreName(name); err != nil {
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
	author, err := config.Author(a.getenv)
	if err != nil {
		return err
	}
	dir := filepath.Join(p.StoresDir, name)
	if _, err := store.Create(dir, &model.StoreMeta{ID: model.NewID(), Name: name}, sc, &model.Workspace{ID: model.NewID(), Name: name, KeyPrefix: "ME"}, author); err != nil {
		return err
	}
	if err := c.AddStore(name, dir); err != nil {
		return errors.Join(err, os.RemoveAll(dir))
	}
	if err := c.Save(); err != nil {
		return errors.Join(err, os.RemoveAll(dir))
	}
	if a.jsonOut {
		return a.printJSON(struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}{name, dir})
	}
	_, err = fmt.Fprintf(a.stdout, "Created store %s at %s\n", name, dir)
	return err
}
