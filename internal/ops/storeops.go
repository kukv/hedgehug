package ops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// StoreInitInput is what StoreInit creates. Workspace is the name of the
// first workspace, the store's name when empty; Schema is the default one
// when nil. Remotes may be empty: the first is origin, the others are
// mirrors. Account is the account that writes the store and makes its GitHub
// repository (the only account when empty). Path is where the store is made
// (under the data directory when empty).
type StoreInitInput struct {
	Name, Prefix, Workspace string
	Schema                  *schema.Schema
	Remotes                 []string
	Account, Path           string
}

// StoreInitResult is the store StoreInit made.
type StoreInitResult struct {
	Name, Path string
	Workspace  *model.Workspace
	Remotes    []string
	Pushed     []string
}

// StoreInit creates a store with one workspace, and with remotes prepares
// them (a bare repository for a local path that is missing, a private
// repository on a GitHub host) and pushes the store to them. A remote on a
// GitHub host with no account to make it is KindUsage, "choice.required",
// with the accounts as candidates. A store that fails halfway leaves nothing
// behind but the GitHub repositories, which the error tells of.
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
	// Account, or the only account. Without either, the store has no account
	// (a remote on a GitHub host then fails in prepareRemotes).
	acct, haveAcct, err := ss.PickAccount(c, in.Account)
	if err != nil {
		return StoreInitResult{}, err
	}
	author, err := config.CommitAuthor(acct, haveAcct, ss.Getenv)
	if err != nil {
		return StoreInitResult{}, err
	}
	accountID := ""
	if haveAcct {
		accountID = acct.ID()
	}
	dir := filepath.Join(p.StoresDir, name)
	if in.Path != "" {
		if dir, err = filepath.Abs(in.Path); err != nil {
			return StoreInitResult{}, err
		}
	}
	e := config.StoreEntry{Name: name, Path: dir, Remotes: in.Remotes}
	if haveAcct {
		e.Account = acct.Name
	}
	made, err := ss.prepareRemotes(c, e, acct, haveAcct)
	if err != nil {
		// made holds what was made before the failure, GitHub repos too.
		return StoreInitResult{}, made.note(err)
	}
	ws := &model.Workspace{ULID: model.NewULID(), Name: wsName, IDPrefix: prefix}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, in.Schema, ws, author, accountID, ss.Now)
	if err != nil {
		return StoreInitResult{}, made.note(errors.Join(err, made.undo()))
	}
	undo := func(err error) error {
		return made.note(errors.Join(err, os.RemoveAll(dir), made.undo())) //nolint:gosec // G703: dir is the store this command just made
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return StoreInitResult{}, undo(err)
	}
	var pushed []string
	if len(in.Remotes) > 0 {
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
	if err := ss.updateConfig(c, func(c *config.Config) error {
		if storeExists(c, name) {
			return storeExistsErr(name)
		}
		if err := c.AddStore(name, dir, in.Remotes); err != nil {
			return err
		}
		// The account that was picked is written, whether Account named it
		// or it was the only one.
		if e.Account != "" {
			return c.SetStoreAccount(name, e.Account)
		}
		return nil
	}); err != nil {
		return StoreInitResult{}, undo(err)
	}
	return StoreInitResult{Name: name, Path: dir, Workspace: ws, Remotes: in.Remotes, Pushed: pushed}, nil
}

// madeRemotes is what prepareRemotes made: local bare repositories, which a
// failure removes, and GitHub repositories, which it leaves and tells of.
type madeRemotes struct {
	bares  []string
	github []string // URLs
	name   string
}

func (m madeRemotes) undo() error {
	var errs []error
	for _, b := range m.bares {
		errs = append(errs, os.RemoveAll(b)) //nolint:gosec // G703: a bare repository this command just made
	}
	return errors.Join(errs...)
}

// note tells of the GitHub repositories left behind.
func (m madeRemotes) note(err error) error {
	if len(m.github) == 0 {
		return err
	}
	return fmt.Errorf("%w\nthe repository %s was made on GitHub and is left there; to use it, run %s",
		err, strings.Join(m.github, ", "), addCommand(m.name, m.github))
}

// githubErr describes err of doing, as acct, and gives it its Kind and Code.
func githubErr(err error, acct config.Account, doing string) error {
	d := github.Describe(err, acct.ID(), acct.Name, doing)
	switch {
	case errors.Is(err, github.ErrUnauthorized):
		return newErr(KindInvalid, "auth.unauthorized", d, "account", acct.Name)
	case errors.Is(err, github.ErrForbidden):
		return newErr(KindInvalid, "github.forbidden", d, "account", acct.Name)
	case errors.Is(err, github.ErrNotFound):
		return newErr(KindInvalid, "github.not_found", d, "account", acct.Name)
	}
	return d
}

// prepareRemotes makes what the remotes of e need before the first push: a
// bare repository for a local path that is missing, a private repository with
// the hedgehug-store topic on a GitHub host. Other remotes must exist
// already. Which kind a remote is, is told by the URL after url_rewrites, and
// a GitHub host is told by its host in any form of a URL (https, ssh://,
// user@host:path).
func (ss *Session) prepareRemotes(c *config.Config, e config.StoreEntry, acct config.Account, haveAcct bool) (madeRemotes, error) {
	m := madeRemotes{name: e.Name}
	for _, u := range e.Remotes {
		to := c.Rewrite(u)
		if p, ok := gitrepo.LocalPath(to); ok {
			if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) { //nolint:gosec // G703: p is a remote written by the person who runs this
				if err := gitrepo.InitBare(p); err != nil {
					return m, errors.Join(err, m.undo())
				}
				m.bares = append(m.bares, p)
			}
			continue
		}
		host, owner, repo, ok := hhctx.Split(to)
		if !ok || !c.IsGitHubHost(host) {
			continue
		}
		if !haveAcct {
			return m, errors.Join(noAccountFor(c, host, u), m.undo())
		}
		if !config.SameHost(host, acct.Host) {
			// The token of the account must not go to another host.
			return m, errors.Join(newErr(KindInvalid, "account.host_mismatch",
				fmt.Errorf("%s is on %s, but account %s is on %s", u, host, acct.Name, acct.Host), "account", acct.Name), m.undo())
		}
		gh, err := ss.GitHub(c, acct, host)
		if err != nil {
			return m, errors.Join(err, m.undo())
		}
		doing := "create " + owner + "/" + repo
		_, err = gh.CreateRepo(owner, repo, strings.EqualFold(owner, acct.Login))
		switch {
		case errors.Is(err, github.ErrExists):
			// 422: the repository is there. One with no ref is used as it is;
			// one with content is a store, or something else, for store add.
			empty, lerr := gitrepo.RemoteEmpty(u, ss.Conn(c, e))
			if lerr != nil {
				return m, errors.Join(fmt.Errorf("%s exists on GitHub, and whether it is empty could not be checked: %w", u, lerr), m.undo())
			}
			if !empty {
				return m, errors.Join(newErr(KindConflict, "store.remote_not_empty",
					fmt.Errorf("%s exists on GitHub and is not empty; to use the store there, run %s", u, addCommand(e.Name, e.Remotes))), m.undo())
			}
		case errors.Is(err, github.ErrForbidden):
			return m, errors.Join(newErr(KindInvalid, "github.forbidden",
				fmt.Errorf("%w\nmake an empty repository on GitHub and give its URL to hedgehug store init --remote; the creation is skipped", githubErr(err, acct, doing)),
				"account", acct.Name), m.undo())
		case err != nil:
			return m, errors.Join(githubErr(err, acct, doing), m.undo())
		default:
			m.github = append(m.github, u)
		}
		if err := gh.SetTopics(owner, repo, []string{"hedgehug-store"}); err != nil {
			return m, errors.Join(githubErr(err, acct, "set the topic of "+owner+"/"+repo), m.undo())
		}
	}
	return m, nil
}

// noAccountFor is the error for the remote u on the GitHub host host when no
// account was picked: a choice among the accounts of that host, or, with none
// on it, the way to set one up.
func noAccountFor(c *config.Config, host, u string) error {
	var names []string
	for _, a := range c.Accounts {
		if config.SameHost(a.Host, host) {
			names = append(names, a.Name)
		}
	}
	if len(names) == 0 {
		return newErr(KindConflict, "account.none",
			fmt.Errorf("%s is on GitHub, so an account of %s makes it; run hedgehug auth login", u, host))
	}
	return choiceRequired("--account", names)
}

// updateConfig is c.Update, after the BeforeUpdate hook of the session (a
// test writes config.yaml there, as another process could).
func (ss *Session) updateConfig(c *config.Config, fn func(*config.Config) error) error {
	if ss.BeforeUpdate != nil {
		ss.BeforeUpdate()
	}
	return c.Update(fn)
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
// remotes, to path (under the data directory when empty), with the account
// named account (none is set when empty, and the only account writes). A
// clone that fails halfway removes what it made. A prefix that a workspace of
// another store has too is warned of. Whether this machine may write to the
// store is checked once it is added; a check that fails is a warning.
func (ss *Session) StoreAdd(name string, remotes []string, path, account string) (StoreAddResult, error) {
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
	// An account that is named must be there, before anything is cloned.
	e := config.StoreEntry{Name: name, Remotes: remotes}
	if account != "" {
		acct, _, err := ss.PickAccount(c, account)
		if err != nil {
			return StoreAddResult{}, err
		}
		e.Account = acct.Name
	}
	dir := filepath.Join(p.StoresDir, name)
	if path != "" {
		if dir, err = filepath.Abs(path); err != nil {
			return StoreAddResult{}, err
		}
	}
	e.Path = dir
	_, statErr := os.Lstat(dir)
	existed := statErr == nil
	// A failed clone removes what it made itself.
	if _, err := gitrepo.CloneWith(remotes[0], dir, ss.Conn(c, e)); err != nil {
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
	// Judge the access before the setup sync, so that a store this machine
	// may not write to is added too: its sync does not push.
	ss.checkAccessOfNew(c, e, s)
	// A store made before the merge driver lacks its .gitattributes; the
	// sync adds it, and pushes it unless the store is read-only (the commit
	// stays here, and is counted as unpushable).
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
			// A push the remote denied makes the store read-only (the sync
			// wrote access.json); it is added as such, not undone.
			if acc, aerr := sync.ReadAccess(s); aerr != nil || !acc.ReadOnly || acc.Reason != sync.ReasonPushDenied {
				return StoreAddResult{}, undo(fmt.Errorf("set up %s: %w", name, err))
			}
			ss.warn(fmt.Sprintf("%s: %v; the setup commit stays here", name, err))
		}
	}
	if err := ss.updateConfig(c, func(c *config.Config) error {
		if storeExists(c, name) {
			return storeExistsErr(name)
		}
		if err := c.AddStore(name, dir, remotes); err != nil {
			return err
		}
		if e.Account != "" {
			return c.SetStoreAccount(name, e.Account)
		}
		return nil
	}); err != nil {
		return StoreAddResult{}, undo(err)
	}
	ss.warnPrefixDuplicates(c, name, s)
	return StoreAddResult{Name: name, Path: dir, Remotes: remotes}, nil
}

// checkAccessOfNew checks whether this machine may write to the store s,
// being added as e, and writes access.json under sync.lock. The store is
// added whatever comes of it: what cannot be checked is a warning.
func (ss *Session) checkAccessOfNew(c *config.Config, e config.StoreEntry, s *store.Store) {
	opt, err := ss.SyncOptions(c, e)
	if err == nil {
		var warns []string
		err = s.WithLocalLock("sync.lock", syncLockWait, func() error {
			var err error
			_, warns, err = sync.CheckAccess(s, opt, ss.Now())
			return err
		})
		for _, w := range warns {
			ss.warn(fmt.Sprintf("%s: %s", e.Name, w))
		}
	}
	if err != nil {
		ss.warn(fmt.Sprintf("%s: the access is not checked: %v", e.Name, err))
	}
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
	if err := ss.updateConfig(c, func(c *config.Config) error {
		if !storeExists(c, name) { // another writer may have removed it since the lookup
			return newErr(KindNotFound, "store.not_found", fmt.Errorf("no store is named %q", name), "name", name)
		}
		return c.RemoveStore(name)
	}); err != nil {
		return StoreRemoveResult{}, err
	}
	if purge {
		if err := os.RemoveAll(e.Path); err != nil {
			return StoreRemoveResult{}, err
		}
	}
	return StoreRemoveResult{Name: name, Path: e.Path, Purged: purge}, nil
}

// StoreDefault makes name the default store and returns the default; an empty
// name changes nothing and returns the current one ("" when there is none).
// config.yaml is not written when name is the default already. A name that
// is no store is KindNotFound, "store.not_found", with the names as
// candidates.
func (ss *Session) StoreDefault(name string) (string, error) {
	c, _, err := ss.Config()
	if err != nil {
		return "", err
	}
	if name == "" || name == c.DefaultStore {
		return c.DefaultStore, nil
	}
	if err := ss.updateConfig(c, func(c *config.Config) error {
		if !storeExists(c, name) { // also when another writer removed it since the read
			names := make([]string, len(c.Stores))
			for i, s := range c.Stores {
				names[i] = s.Name
			}
			msg := fmt.Sprintf("no store is named %q", name)
			if len(names) > 0 {
				msg += " (stores: " + strings.Join(names, ", ") + ")"
			}
			return newErr(KindNotFound, "store.not_found", errors.New(msg), "name", name, "candidates", names)
		}
		return c.SetDefaultStore(name)
	}); err != nil {
		return "", err
	}
	return c.DefaultStore, nil
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
