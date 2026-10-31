package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
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
	// The new store has no entry yet, so only the single-account fallback applies.
	acct, ok := c.Writer(config.StoreEntry{Name: name})
	author, err := config.CommitAuthor(acct, ok, a.getenv)
	if err != nil {
		return err
	}
	account := ""
	if ok {
		account = acct.ID()
	}
	dir := filepath.Join(p.StoresDir, name)
	ws := &model.Workspace{ULID: model.NewULID(), Name: wsName, IDPrefix: prefix}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, sc, ws, author, account, a.now)
	if err != nil {
		return err
	}
	undo := func(err error) error { return errors.Join(err, os.RemoveAll(dir)) }
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return undo(err)
	}
	var pushed []string
	if len(remotes) > 0 {
		e := config.StoreEntry{Name: name, Path: dir, Remotes: remotes}
		opt, err := a.syncOptions(c, e)
		if err != nil {
			return undo(err)
		}
		res, err := a.runSync(s, opt)
		if errors.Is(err, gitrepo.ErrRejected) || errors.Is(err, sync.ErrUnrelated) {
			err = fmt.Errorf("the remote is not empty; to use the store there, run %s", addCommand(name, remotes))
		}
		if err != nil {
			return undo(err)
		}
		pushed = res.Pushed
		for _, w := range res.Warnings {
			_, _ = fmt.Fprintf(a.stderr, "warning: %s: %s\n", name, w)
		}
	}
	if err := c.AddStore(name, dir, remotes); err != nil {
		return undo(err)
	}
	if err := c.Save(); err != nil {
		return undo(err)
	}
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
		}{name, dir, wsJSON{ws.ULID.String(), ws.Name, ws.IDPrefix}, remotes, pushed})
	}
	_, err = fmt.Fprintf(a.stdout, "Created store %s at %s with workspace %s (%s)\n", name, dir, ws.Name, ws.IDPrefix)
	if err == nil && len(pushed) > 0 {
		_, err = fmt.Fprintf(a.stdout, "Pushed to %s\n", strings.Join(pushed, ", "))
	}
	return err
}

// addCommand is the hedgehug store add that takes the store of the remotes.
func addCommand(name string, remotes []string) string {
	var b strings.Builder
	b.WriteString("hedgehug store add " + name)
	for _, r := range remotes {
		b.WriteString(" --remote " + r)
	}
	return b.String()
}
