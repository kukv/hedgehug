package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

func (a *app) storeAddCmd() *cobra.Command {
	var remotes []string
	var path string
	cmd := &cobra.Command{
		Use:   "add <name> --remote <url>...",
		Short: "Add a store that others made, by cloning it",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(remotes) == 0 {
				return usageErrorf("--remote is required: the remote to clone the store from")
			}
			return a.storeAdd(args[0], remotes, path)
		},
	}
	cmd.Flags().StringArrayVar(&remotes, "remote", nil, "a remote of the store; the first is cloned, the others are mirrors")
	cmd.Flags().StringVar(&path, "path", "", "where to clone the store (default: under the data directory, as store init does)")
	return cmd
}

func (a *app) storeAdd(name string, remotes []string, path string) error {
	if err := config.ValidStoreName(name); err != nil {
		return usageError{err}
	}
	c, p, err := a.ss.Config()
	if err != nil {
		return err
	}
	for _, e := range c.Stores {
		if e.Name == name {
			return fmt.Errorf("a store named %q is already set up", name)
		}
	}
	dir := filepath.Join(p.StoresDir, name)
	if path != "" {
		if dir, err = filepath.Abs(path); err != nil {
			return err
		}
	}
	_, statErr := os.Lstat(dir)
	existed := statErr == nil
	// A failed clone removes what it made itself.
	if _, err := gitrepo.Clone(remotes[0], dir); err != nil {
		return err
	}
	undo := func(err error) error { return errors.Join(err, clearDir(dir, existed)) }
	s, err := store.Open(dir)
	if err != nil {
		return undo(fmt.Errorf("%s is not a hedgehug store: %w", remotes[0], err))
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return undo(err)
	}
	if err := s.Repo().SetRemotes(remotes); err != nil {
		return undo(err)
	}
	e := config.StoreEntry{Name: name, Path: dir, Remotes: remotes}
	// A store made before the merge driver lacks its .gitattributes; the
	// sync adds it, and pushes it.
	need, err := sync.NeedsSetup(s)
	if err != nil {
		return undo(err)
	}
	if need {
		opt, err := a.ss.SyncOptions(c, e)
		if err != nil {
			return undo(err)
		}
		if _, err := a.ss.RunSync(s, opt, true); err != nil {
			return undo(fmt.Errorf("set up %s: %w", name, err))
		}
	}
	if err := c.AddStore(name, dir, remotes); err != nil {
		return undo(err)
	}
	if err := c.Save(); err != nil {
		return undo(err)
	}
	a.warnPrefixDuplicates(c, name, s)
	if a.jsonOut {
		return a.printJSON(struct {
			Name    string   `json:"name"`
			Path    string   `json:"path"`
			Remotes []string `json:"remotes"`
		}{name, dir, remotes})
	}
	_, err = fmt.Fprintf(a.stdout, "Added store %s at %s from %s\n", name, dir, remotes[0])
	return err
}

// clearDir takes back what a failed command made in dir: dir itself, or only
// what is in it when it existed before.
func clearDir(dir string, existed bool) error {
	if !existed {
		return os.RemoveAll(dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		errs = append(errs, os.RemoveAll(filepath.Join(dir, e.Name())))
	}
	return errors.Join(errs...)
}

// warnPrefixDuplicates tells on stderr of each workspace of the store s,
// just added to c as name, whose id prefix a workspace of another store has
// too. The prefix that was set later is the one to change. A store that
// cannot be opened is skipped with a warning.
func (a *app) warnPrefixDuplicates(c *config.Config, name string, s *store.Store) {
	var mine []*model.Workspace
	var prefixes []string
	for w, err := range s.Workspaces() {
		if err == nil && w.IDPrefix != "" {
			mine = append(mine, w)
			prefixes = append(prefixes, w.IDPrefix)
		}
	}
	var others []store.NamedStore
	for _, e := range c.Stores {
		if e.Name != name {
			others = append(others, store.NamedStore{Name: e.Name, Path: e.Path})
		}
	}
	uses := store.PrefixUses(others, prefixes, func(n string, err error) {
		_, _ = fmt.Fprintf(a.stderr, "warning: store %s is not checked for id prefixes: %v\n", n, err)
	})
	for _, m := range mine {
		for _, u := range uses {
			w := u.Workspace
			if !strings.EqualFold(m.IDPrefix, w.IDPrefix) {
				continue
			}
			later := fmt.Sprintf("workspace %s in store %s", m.Name, name)
			if !setLater(m, w) {
				later = fmt.Sprintf("workspace %s in store %s", w.Name, u.Store)
			}
			_, _ = fmt.Fprintf(a.stderr, "warning: %s is also the id prefix of workspace %s in store %s; the one set later (%s) should change it with hedgehug workspace prefix\n",
				m.IDPrefix, w.Name, u.Store, later)
		}
	}
}

// setLater reports whether the id prefix of m was set after that of w; set
// at the same time, the larger ULID is the later.
func setLater(m, w *model.Workspace) bool {
	cmp := m.PrefixSet.Compare(w.PrefixSet)
	return cmp > 0 || cmp == 0 && m.ULID.String() > w.ULID.String()
}

func (a *app) storeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the stores that are set up",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.storeList() },
	}
}

// storeJSON is a store of config.yaml as store list --json prints it.
type storeJSON struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Remotes     []string   `json:"remotes"`
	Account     string     `json:"account,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"` // of the last sync
}

func (a *app) storeList() error {
	c, _, err := a.ss.Config()
	if err != nil {
		return err
	}
	out := make([]storeJSON, len(c.Stores))
	for i, e := range c.Stores {
		out[i] = storeJSON{Name: e.Name, Path: e.Path, Remotes: append([]string{}, e.Remotes...)}
		if acct, ok := c.Writer(e); ok {
			out[i].Account = acct.ID()
		}
		// A store that does not open or has no state is listed without its last sync.
		if s, err := store.Open(e.Path); err == nil {
			if st, err := sync.ReadState(s); err == nil {
				out[i].LastSuccess = timePtr(st.LastSuccess)
			}
		}
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
	c, _, err := a.ss.Config()
	if err != nil {
		return err
	}
	e, err := c.Select(name)
	if err != nil {
		return err
	}
	if purge {
		if err := checkPushed(e); err != nil {
			return err
		}
	}
	if err := c.RemoveStore(name); err != nil {
		return err
	}
	if err := c.Save(); err != nil {
		return err
	}
	if purge {
		if err := os.RemoveAll(e.Path); err != nil {
			return err
		}
	}
	if a.jsonOut {
		return a.printJSON(struct {
			Name   string `json:"name"`
			Path   string `json:"path"`
			Purged bool   `json:"purged"`
		}{name, e.Path, purge})
	}
	if purge {
		_, err = fmt.Fprintf(a.stdout, "Removed store %s and deleted %s\n", name, e.Path)
	} else {
		_, err = fmt.Fprintf(a.stdout, "Removed store %s from the config; %s is left\n", name, e.Path)
	}
	return err
}

// checkPushed refuses to delete a store whose commits are not all on its
// origin, or that has no origin to hold them.
func checkPushed(e config.StoreEntry) error {
	if len(e.Remotes) == 0 {
		return fmt.Errorf("store %s has no remote, so its commits are only in %s; delete the directory by hand if you mean to lose them", e.Name, e.Path)
	}
	s, err := store.Open(e.Path)
	if err != nil {
		return fmt.Errorf("store %s: %w; delete the directory by hand if you mean to", e.Name, err)
	}
	n, err := ops.Unpushed(s)
	if err != nil {
		return err
	}
	st, err := s.Repo().Status()
	if err != nil {
		return err
	}
	if len(st) > 0 {
		paths := make([]string, len(st))
		for i, f := range st {
			paths[i] = f.Path
		}
		return fmt.Errorf("store %s has changes that are not committed (%s); commit or restore them, or delete the directory by hand if you mean to lose them", e.Name, strings.Join(paths, ", "))
	}
	if n > 0 {
		return fmt.Errorf("store %s has %d commits not pushed; run hedgehug sync --store %s first", e.Name, n, e.Name)
	}
	return nil
}
