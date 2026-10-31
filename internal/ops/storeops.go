package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// StoreInitInput is what StoreInit creates. Workspace is the name of the
// first workspace, the store's name when empty; Schema is the default one
// when nil. Remotes must be empty: the first is origin, the others mirrors.
type StoreInitInput struct {
	Name, Prefix, Workspace string
	Schema                  *schema.Schema
	Remotes                 []string
}

// StoreInitResult is the store StoreInit made.
type StoreInitResult struct {
	Name, Path string
	Workspace  *model.Workspace
	Remotes    []string
	Pushed     []string
}

// StoreInit creates a store with one workspace, and with remotes pushes it
// to them. A store that fails halfway leaves nothing behind.
func (ss *Session) StoreInit(in StoreInitInput) (StoreInitResult, error) {
	name, prefix, wsName := in.Name, in.Prefix, in.Workspace
	if err := config.ValidStoreName(name); err != nil {
		return StoreInitResult{}, newErr(KindUsage, "store.bad_name", err, "value", name)
	}
	if err := badIDPrefix(prefix); err != nil {
		return StoreInitResult{}, err
	}
	if wsName == "" {
		wsName = name
	}
	if err := badWorkspaceName(wsName); err != nil {
		return StoreInitResult{}, err
	}
	c, p, err := ss.Config()
	if err != nil {
		return StoreInitResult{}, err
	}
	if storeExists(c, name) {
		return StoreInitResult{}, storeExistsErr(name)
	}
	if err := ss.CheckPrefixElsewhere(prefix, name); err != nil {
		return StoreInitResult{}, err
	}
	// The new store has no entry yet, so only the single-account fallback applies.
	acct, ok := c.Writer(config.StoreEntry{Name: name})
	author, err := config.CommitAuthor(acct, ok, ss.Getenv)
	if err != nil {
		return StoreInitResult{}, err
	}
	account := ""
	if ok {
		account = acct.ID()
	}
	dir := filepath.Join(p.StoresDir, name)
	ws := &model.Workspace{ULID: model.NewULID(), Name: wsName, IDPrefix: prefix}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, in.Schema, ws, author, account, ss.Now)
	if err != nil {
		return StoreInitResult{}, err
	}
	undo := func(err error) error { return errors.Join(err, os.RemoveAll(dir)) }
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return StoreInitResult{}, undo(err)
	}
	var pushed []string
	if len(in.Remotes) > 0 {
		e := config.StoreEntry{Name: name, Path: dir, Remotes: in.Remotes}
		opt, err := ss.SyncOptions(c, e)
		if err != nil {
			return StoreInitResult{}, undo(err)
		}
		res, err := ss.RunSync(s, opt, true)
		if errors.Is(err, gitrepo.ErrRejected) || errors.Is(err, sync.ErrUnrelated) {
			err = newErr(KindConflict, "store.remote_not_empty",
				fmt.Errorf("the remote is not empty; to use the store there, run %s", addCommand(name, in.Remotes)))
		}
		if err != nil {
			return StoreInitResult{}, undo(err)
		}
		pushed = res.Pushed
		for _, w := range res.Warnings {
			ss.warn(fmt.Sprintf("%s: %s", name, w))
		}
	}
	if err := c.AddStore(name, dir, in.Remotes); err != nil {
		return StoreInitResult{}, undo(err)
	}
	if err := c.Save(); err != nil {
		return StoreInitResult{}, undo(err)
	}
	return StoreInitResult{Name: name, Path: dir, Workspace: ws, Remotes: in.Remotes, Pushed: pushed}, nil
}

func storeExists(c *config.Config, name string) bool {
	for _, e := range c.Stores {
		if e.Name == name {
			return true
		}
	}
	return false
}

func storeExistsErr(name string) error {
	return newErr(KindConflict, "store.exists", fmt.Errorf("a store named %q is already set up", name), "name", name)
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

// StoreAddResult is the store StoreAdd cloned, as store add --json prints it.
type StoreAddResult struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Remotes []string `json:"remotes"`
}

// StoreAdd adds the store that others made by cloning it from the first of
// remotes, to path (under the data directory when empty). A clone that fails
// halfway removes what it made. A prefix that a workspace of another store
// has too is warned of.
func (ss *Session) StoreAdd(name string, remotes []string, path string) (StoreAddResult, error) {
	if len(remotes) == 0 {
		return StoreAddResult{}, newErr(KindUsage, "store.remote_required", errors.New("a remote is needed"))
	}
	if err := config.ValidStoreName(name); err != nil {
		return StoreAddResult{}, newErr(KindUsage, "store.bad_name", err, "value", name)
	}
	c, p, err := ss.Config()
	if err != nil {
		return StoreAddResult{}, err
	}
	if storeExists(c, name) {
		return StoreAddResult{}, storeExistsErr(name)
	}
	dir := filepath.Join(p.StoresDir, name)
	if path != "" {
		if dir, err = filepath.Abs(path); err != nil {
			return StoreAddResult{}, err
		}
	}
	_, statErr := os.Lstat(dir)
	existed := statErr == nil
	// A failed clone removes what it made itself.
	if _, err := gitrepo.CloneWith(remotes[0], dir, ss.conn(c, config.StoreEntry{Name: name, Path: dir, Remotes: remotes})); err != nil {
		// KindInvalid, not KindUsage: the CLI exits 1.
		return StoreAddResult{}, newErr(KindInvalid, "store.clone_failed", err, "remote", remotes[0])
	}
	undo := func(err error) error { return errors.Join(err, clearDir(dir, existed)) }
	s, err := ss.openStoreAt(dir)
	if err != nil {
		return StoreAddResult{}, undo(newErr(KindInvalid, "store.not_hedgehug", fmt.Errorf("%s is not a hedgehug store: %w", remotes[0], err), "name", name))
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return StoreAddResult{}, undo(err)
	}
	if err := s.Repo().SetRemotes(remotes); err != nil {
		return StoreAddResult{}, undo(err)
	}
	e := config.StoreEntry{Name: name, Path: dir, Remotes: remotes}
	// A store made before the merge driver lacks its .gitattributes; the
	// sync adds it, and pushes it.
	need, err := sync.NeedsSetup(s)
	if err != nil {
		return StoreAddResult{}, undo(err)
	}
	if need {
		opt, err := ss.SyncOptions(c, e)
		if err != nil {
			return StoreAddResult{}, undo(err)
		}
		if _, err := ss.RunSync(s, opt, true); err != nil {
			return StoreAddResult{}, undo(fmt.Errorf("set up %s: %w", name, err))
		}
	}
	if err := c.AddStore(name, dir, remotes); err != nil {
		return StoreAddResult{}, undo(err)
	}
	if err := c.Save(); err != nil {
		return StoreAddResult{}, undo(err)
	}
	ss.warnPrefixDuplicates(c, name, s)
	return StoreAddResult{Name: name, Path: dir, Remotes: remotes}, nil
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

// warnPrefixDuplicates warns of each workspace of the store s, just added to
// c as name, whose id prefix a workspace of another store has too. The prefix
// that was set later is the one to change. A store that cannot be opened is
// skipped with a warning.
func (ss *Session) warnPrefixDuplicates(c *config.Config, name string, s *store.Store) {
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
		ss.warn(fmt.Sprintf("store %s is not checked for id prefixes: %v", n, err))
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
			ss.warn(fmt.Sprintf("%s is also the id prefix of workspace %s in store %s; the one set later (%s) should change it with hedgehug workspace prefix",
				m.IDPrefix, w.Name, u.Store, later))
		}
	}
}

// setLater reports whether the id prefix of m was set after that of w; set
// at the same time, the larger ULID is the later.
func setLater(m, w *model.Workspace) bool {
	cmp := m.PrefixSet.Compare(w.PrefixSet)
	return cmp > 0 || cmp == 0 && m.ULID.String() > w.ULID.String()
}

// StoreView is a store of config.yaml as store list --json prints it.
type StoreView struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Remotes     []string   `json:"remotes"`
	Account     string     `json:"account,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"` // of the last sync
}

// StoreList lists the stores that are set up.
func (ss *Session) StoreList() ([]StoreView, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	out := make([]StoreView, len(c.Stores))
	for i, e := range c.Stores {
		out[i] = StoreView{Name: e.Name, Path: e.Path, Remotes: append([]string{}, e.Remotes...)}
		if acct, ok := c.Writer(e); ok {
			out[i].Account = acct.ID()
		}
		// A store that does not open or has no state is listed without its last sync.
		if s, err := ss.openStoreAt(e.Path); err == nil {
			if st, err := sync.ReadState(s); err == nil {
				out[i].LastSuccess = timePtr(st.LastSuccess)
			}
		}
	}
	return out, nil
}

// StoreRemoveResult is the store StoreRemove took out of the config, as
// store remove --json prints it.
type StoreRemoveResult struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Purged bool   `json:"purged"`
}

// StoreRemove takes the store name out of config.yaml and leaves its
// directory. With purge it deletes the directory too, unless commits are not
// pushed yet.
func (ss *Session) StoreRemove(name string, purge bool) (StoreRemoveResult, error) {
	c, _, err := ss.Config()
	if err != nil {
		return StoreRemoveResult{}, err
	}
	e, err := c.Select(name)
	if err != nil {
		if errors.Is(err, config.ErrNoStores) {
			return StoreRemoveResult{}, err
		}
		return StoreRemoveResult{}, newErr(KindNotFound, "store.not_found", err, "name", name)
	}
	if purge {
		if err := ss.checkPushed(e); err != nil {
			return StoreRemoveResult{}, err
		}
	}
	if err := c.RemoveStore(name); err != nil {
		return StoreRemoveResult{}, err
	}
	if err := c.Save(); err != nil {
		return StoreRemoveResult{}, err
	}
	if purge {
		if err := os.RemoveAll(e.Path); err != nil {
			return StoreRemoveResult{}, err
		}
	}
	return StoreRemoveResult{Name: name, Path: e.Path, Purged: purge}, nil
}

// checkPushed refuses to delete a store whose commits are not all on its
// origin, or that has no origin to hold them.
func (ss *Session) checkPushed(e config.StoreEntry) error {
	unpushed := func(err error) error { return newErr(KindConflict, "store.unpushed", err, "name", e.Name) }
	if len(e.Remotes) == 0 {
		return unpushed(fmt.Errorf("store %s has no remote, so its commits are only in %s; delete the directory by hand if you mean to lose them", e.Name, e.Path))
	}
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return unpushed(fmt.Errorf("store %s: %w; delete the directory by hand if you mean to", e.Name, err))
	}
	n, err := Unpushed(s)
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
		return unpushed(fmt.Errorf("store %s has changes that are not committed (%s); commit or restore them, or delete the directory by hand if you mean to lose them", e.Name, strings.Join(paths, ", ")))
	}
	if n > 0 {
		return unpushed(fmt.Errorf("store %s has %d commits not pushed; run hedgehug sync --store %s first", e.Name, n, e.Name))
	}
	return nil
}
