// Package stores sets the stores up: init, add, remove, discover, the move
// of tasks to another store, and hh use.
package stores

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/github"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/giturl"
	"github.com/kukv/hedgehug/internal/lib/osfs"
	"github.com/kukv/hedgehug/internal/lib/plural"
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
	// Automations, when given, is written as automations.yaml right after
	// the store is made, in a commit of op init.
	Automations   *automation.File
	Remotes       []string
	Account, Path string
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
func StoreInit(ss *usecase.Session, in StoreInitInput) (StoreInitResult, error) {
	name, prefix, wsName := in.Name, in.Prefix, in.Workspace
	if err := config.ValidStoreName(name); err != nil {
		return StoreInitResult{}, usecase.NewErr(usecase.KindUsage, "store.bad_name", err, "value", name)
	}
	if err := usecase.BadIDPrefix(prefix); err != nil {
		return StoreInitResult{}, err
	}
	if wsName == "" {
		wsName = name
	}
	if err := usecase.BadWorkspaceName(wsName); err != nil {
		return StoreInitResult{}, err
	}
	c, p, err := ss.Config()
	if err != nil {
		return StoreInitResult{}, err
	}
	if usecase.StoreExists(c, name) {
		return StoreInitResult{}, storeExistsErr(name)
	}
	if err := ss.CheckPrefixElsewhere(prefix, name); err != nil {
		return StoreInitResult{}, err
	}
	// Account, or the only account. With neither, a remote on a GitHub host
	// fails in prepareRemotes; any other store is stopped after it.
	acct, haveAcct, err := ss.PickAccount(c, in.Account)
	if err != nil {
		return StoreInitResult{}, err
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
	made, err := prepareRemotes(ss, c, e, acct, haveAcct)
	if err != nil {
		// made holds what was made before the failure, GitHub repos too.
		return StoreInitResult{}, made.note(err)
	}
	if !haveAcct {
		// No remote asked for an account; a store needs one all the same.
		return StoreInitResult{}, made.note(errors.Join(noAccountChoice(c), made.undo()))
	}
	author, accountID := config.CommitAuthor(acct), acct.ID()
	ws := &model.Workspace{ULID: model.NewULID(), Name: wsName, IDPrefix: prefix}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, in.Schema, ws, author, accountID, ss.Now)
	if err != nil {
		return StoreInitResult{}, made.note(errors.Join(err, made.undo()))
	}
	defer func() { _ = s.Repo().Close() }()
	undo := func(err error) error {
		_ = s.Repo().Close() // before the files go (Windows)
		return made.note(errors.Join(err, osfs.RemoveAll(dir), made.undo()))
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return StoreInitResult{}, undo(err)
	}
	if in.Automations != nil {
		data, err := in.Automations.Encode()
		if err != nil {
			return StoreInitResult{}, undo(err)
		}
		if _, err := s.UpdateCommit(store.Op{Name: "init", Author: author, Account: accountID}, func(tx *store.Tx) error {
			return tx.PutFile(model.AutomationsPath, data)
		}); err != nil {
			return StoreInitResult{}, undo(err)
		}
	}
	var pushed []string
	if len(in.Remotes) > 0 {
		opt, err := ss.SyncOptions(c, e)
		if err != nil {
			return StoreInitResult{}, undo(err)
		}
		res, err := ss.RunSync(s, opt, true)
		if errors.Is(err, gitrepo.ErrRejected) || errors.Is(err, replicate.ErrUnrelated) {
			err = usecase.NewErr(usecase.KindConflict, "store.remote_not_empty",
				fmt.Errorf("the remote is not empty; to use the store there, run %s", addCommand(name, in.Remotes)))
		}
		if err != nil {
			return StoreInitResult{}, undo(err)
		}
		pushed = res.Pushed
		for _, w := range res.Warnings {
			ss.AddWarning(fmt.Sprintf("%s: %s", name, w))
		}
	}
	if err := ss.UpdateConfig(c, func(c *config.Config) error {
		if usecase.StoreExists(c, name) {
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
		// The remotes kept what was pushed, though the store here is removed:
		// the way to use it comes with the error.
		if kept := made.keptWithCommits(c, in.Remotes, len(pushed) > 0); len(kept) > 0 {
			err = fmt.Errorf("%w\nthe commits were pushed to %s and stay there; to use them, run %s",
				err, strings.Join(kept, ", "), addCommand(name, kept))
		}
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
		errs = append(errs, osfs.RemoveAll(b))
	}
	return errors.Join(errs...)
}

// keptWithCommits is the remotes among remotes that hold the commits of a
// store that was pushed, and are not removed by undo or told of by note: the
// ones that were there before the command (an empty GitHub repository, a local
// bare).
func (m madeRemotes) keptWithCommits(c *config.Config, remotes []string, pushed bool) []string {
	if !pushed {
		return nil
	}
	var kept []string

	for _, u := range remotes {
		if slices.Contains(m.github, u) {
			continue
		}
		if p, ok := gitrepo.LocalPath(c.Rewrite(u)); ok && slices.Contains(m.bares, p) {
			continue
		}
		kept = append(kept, u)
	}
	return kept
}

// note tells of the GitHub repositories left behind.
func (m madeRemotes) note(err error) error {
	if len(m.github) == 0 {
		return err
	}
	return fmt.Errorf("%w\nthe repository %s was made on GitHub and is left there; to use it, run %s",
		err, strings.Join(m.github, ", "), addCommand(m.name, m.github))
}

// prepareRemotes makes what the remotes of e need before the first push: a
// bare repository for a local path that is missing, a private repository with
// the hedgehug-store topic on a GitHub host. Other remotes must exist
// already. Which kind a remote is, is told by the URL after url_rewrites, and
// a GitHub host is told by its host in any form of a URL (https, ssh://,
// user@host:path).
func prepareRemotes(ss *usecase.Session, c *config.Config, e config.StoreEntry, acct config.Account, haveAcct bool) (madeRemotes, error) {
	m := madeRemotes{name: e.Name}
	for _, u := range e.Remotes {
		to := c.Rewrite(u)
		if p, ok := gitrepo.LocalPath(to); ok {
			if _, err := osfs.Stat(p); errors.Is(err, fs.ErrNotExist) { //nolint:gosec // G703: p is a remote written by the person who runs this
				if err := gitrepo.InitBare(p); err != nil {
					return m, errors.Join(err, m.undo())
				}
				m.bares = append(m.bares, p)
			}
			continue
		}
		host, owner, repo, ok := giturl.Split(to)
		if !ok || !c.IsGitHubHost(host) {
			continue
		}
		if !haveAcct {
			return m, errors.Join(noAccountFor(c, host, u), m.undo())
		}
		if !config.SameHost(host, acct.Host) {
			// The token of the account must not go to another host.
			return m, errors.Join(usecase.NewErr(usecase.KindInvalid, "account.host_mismatch",
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
				return m, errors.Join(usecase.NewErr(usecase.KindConflict, "store.remote_not_empty",
					fmt.Errorf("%s exists on GitHub and is not empty; to use the store there, run %s", u, addCommand(e.Name, e.Remotes))), m.undo())
			}
		case errors.Is(err, github.ErrForbidden):
			return m, errors.Join(usecase.NewErr(usecase.KindInvalid, "github.forbidden",
				fmt.Errorf("%w\nmake an empty repository on GitHub and give its URL to hedgehug store init --remote; the creation is skipped", usecase.GithubErr(err, acct, doing)),
				"account", acct.Name), m.undo())
		case err != nil:
			return m, errors.Join(usecase.GithubErr(err, acct, doing), m.undo())
		default:
			m.github = append(m.github, u)
		}
		if err := gh.SetTopics(owner, repo, []string{"hedgehug-store"}); err != nil {
			return m, errors.Join(usecase.GithubErr(err, acct, "set the topic of "+owner+"/"+repo), m.undo())
		}
	}
	return m, nil
}

// noAccountChoice is the error for a store made with no account named while
// c has two or more: the choice among them. With none at all it is
// ErrNoAccounts.
func noAccountChoice(c *config.Config) error {
	if len(c.Accounts) == 0 {
		return usecase.ErrNoAccounts
	}
	names := make([]string, len(c.Accounts))
	for i, a := range c.Accounts {
		names[i] = a.Name
	}
	return usecase.ChoiceRequired("--account", names)
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
		return usecase.NewErr(usecase.KindConflict, "account.none",
			fmt.Errorf("%s is on GitHub, so an account of %s makes it; run hedgehug auth login", u, host))
	}
	return usecase.ChoiceRequired("--account", names)
}

func storeExistsErr(name string) error {
	return usecase.NewErr(usecase.KindConflict, "store.exists", fmt.Errorf("a store named %q is already set up", name), "name", name)
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
// named account (the only account when empty; with two or more it is
// needed). A clone that fails halfway removes what it made. A prefix that a
// workspace of another store has too is warned of. Whether this machine may
// write to the store is checked once it is added; a check that fails is a
// warning.
func StoreAdd(ss *usecase.Session, name string, remotes []string, path, account string) (StoreAddResult, error) {
	if len(remotes) == 0 {
		return StoreAddResult{}, usecase.NewErr(usecase.KindUsage, "store.remote_required", errors.New("a remote is needed"))
	}
	if err := config.ValidStoreName(name); err != nil {
		return StoreAddResult{}, usecase.NewErr(usecase.KindUsage, "store.bad_name", err, "value", name)
	}
	c, p, err := ss.Config()
	if err != nil {
		return StoreAddResult{}, err
	}
	if usecase.StoreExists(c, name) {
		return StoreAddResult{}, storeExistsErr(name)
	}
	// An account that is named must be there, before anything is cloned.
	e := config.StoreEntry{Name: name, Remotes: remotes}
	acct, ok, err := ss.PickAccount(c, account)
	if err != nil {
		return StoreAddResult{}, err
	}
	if !ok {
		return StoreAddResult{}, noAccountChoice(c)
	}
	if account != "" {
		e.Account = acct.Name
	}
	dir := filepath.Join(p.StoresDir, name)
	if path != "" {
		if dir, err = filepath.Abs(path); err != nil {
			return StoreAddResult{}, err
		}
	}
	e.Path = dir
	_, statErr := osfs.Lstat(dir)
	existed := statErr == nil
	// A failed clone removes what it made itself.
	cloned, err := gitrepo.CloneWith(remotes[0], dir, ss.Conn(c, e))
	if err != nil {
		// KindInvalid, not KindUsage: the CLI exits 1.
		return StoreAddResult{}, usecase.NewErr(usecase.KindInvalid, "store.clone_failed", err, "remote", remotes[0])
	}
	_ = cloned.Close() // the store is opened below
	// The handle is closed before the directory goes: Windows cannot remove
	// open pack files.
	var s *store.Store
	undo := func(err error) error {
		if s != nil {
			_ = s.Repo().Close()
		}
		return errors.Join(err, clearDir(dir, existed))
	}
	s, err = ss.OpenStoreAt(dir)
	if err != nil {
		return StoreAddResult{}, undo(usecase.NewErr(usecase.KindInvalid, "store.not_hedgehug", fmt.Errorf("%s is not a hedgehug store: %w", remotes[0], err), "name", name))
	}
	defer func() { _ = s.Repo().Close() }()
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return StoreAddResult{}, undo(err)
	}
	if err := s.Repo().SetRemotes(remotes); err != nil {
		return StoreAddResult{}, undo(err)
	}
	// Judge the access before the setup sync, so that a store this machine
	// may not write to is added too: its sync does not push.
	checkAccessOfNew(ss, c, e, s)
	// A store made before the merge driver lacks its .gitattributes; the
	// sync adds it, and pushes it unless the store is read-only (the commit
	// stays here, and is counted as unpushable).
	need, err := replicate.NeedsSetup(s)
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
			if acc, aerr := replicate.ReadAccess(s); aerr != nil || !acc.ReadOnly || acc.Reason != replicate.ReasonPushDenied {
				return StoreAddResult{}, undo(fmt.Errorf("set up %s: %w", name, err))
			}
			ss.AddWarning(fmt.Sprintf("%s: %v; the setup commit stays here", name, err))
		}
	}
	if err := ss.UpdateConfig(c, func(c *config.Config) error {
		if usecase.StoreExists(c, name) {
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
	// The index is a cache: a store it cannot be built for is added all the
	// same, and the next command that needs it builds it again.
	if _, err := ss.OpenHistory(s, name); err != nil {
		ss.AddWarning(fmt.Sprintf("%s: could not index its history: %v", name, err))
	}
	warnPrefixDuplicates(ss, c, name, s)
	return StoreAddResult{Name: name, Path: dir, Remotes: remotes}, nil
}

// checkAccessOfNew checks whether this machine may write to the store s,
// being added as e, and writes access.json under sync.lock. The store is
// added whatever comes of it: what cannot be checked is a warning.
func checkAccessOfNew(ss *usecase.Session, c *config.Config, e config.StoreEntry, s *store.Store) {
	opt, err := ss.SyncOptions(c, e)
	if err == nil {
		var warns []string
		err = s.WithLocalLock("sync.lock", usecase.SyncLockWait, func() error {
			var err error
			_, warns, err = replicate.CheckAccess(s, opt, ss.Now())
			return err
		})
		for _, w := range warns {
			ss.AddWarning(fmt.Sprintf("%s: %s", e.Name, w))
		}
	}
	if err != nil {
		ss.AddWarning(fmt.Sprintf("%s: the access is not checked: %v", e.Name, err))
	}
}

// clearDir takes back what a failed command made in dir: dir itself, or only
// what is in it when it existed before.
func clearDir(dir string, existed bool) error {
	if !existed {
		return osfs.RemoveAll(dir)
	}
	entries, err := osfs.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		errs = append(errs, osfs.RemoveAll(filepath.Join(dir, e.Name())))
	}
	return errors.Join(errs...)
}

// warnPrefixDuplicates warns of each workspace of the store s, just added to
// c as name, whose id prefix a workspace of another store has too. The prefix
// that was set later is the one to change. A store that cannot be opened is
// skipped with a warning.
func warnPrefixDuplicates(ss *usecase.Session, c *config.Config, name string, s *store.Store) {
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
		ss.AddWarning(fmt.Sprintf("store %s is not checked for id prefixes: %v", n, err))
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
			ss.AddWarning(fmt.Sprintf("%s is also the id prefix of workspace %s in store %s; the one set later (%s) should change it with hedgehug workspace set-prefix",
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
	Name         string     `json:"name"`
	Path         string     `json:"path"`
	Remotes      []string   `json:"remotes"`
	Account      string     `json:"account,omitempty"`
	Color        string     `json:"color,omitempty"`        // from config.yaml; empty: the GUI picks one by order
	Hidden       bool       `json:"hidden"`                 // from config.yaml; only the GUI leaves the store out
	ReadOnly     string     `json:"read_only,omitempty"`    // why this machine cannot write the store, in words; absent when it can
	DefaultStore bool       `json:"default_store"`          // config.yaml's default_store names this store
	LastSuccess  *time.Time `json:"last_success,omitempty"` // of the last sync
}

// StoreList lists the stores that are set up.
func StoreList(ss *usecase.Session) ([]StoreView, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	out := make([]StoreView, len(c.Stores))
	for i, e := range c.Stores {
		out[i] = storeView(ss, c, e)
	}
	return out, nil
}

// storeView is the view of the store e of c. A store that does not open or
// has no state is listed without its last sync and its read-only reason.
func storeView(ss *usecase.Session, c *config.Config, e config.StoreEntry) StoreView {
	v := StoreView{Name: e.Name, Path: e.Path, Remotes: append([]string{}, e.Remotes...), Color: e.Color, Hidden: e.Hidden, DefaultStore: c.DefaultStore == e.Name}
	if acct, ok := c.Writer(e); ok {
		v.Account = acct.ID()
	}
	if s, err := ss.OpenStoreAt(e.Path); err == nil {
		if st, err := replicate.ReadState(s); err == nil {
			v.LastSuccess = usecase.TimePtr(st.LastSuccess)
		}
		v.ReadOnly = usecase.ReadOnlyWhy(s)
		_ = s.Repo().Close()
	}
	return v
}

// StoreRemoveResult is the store StoreRemove took out of the config, as
// store remove --json prints it. Deleted is whether the directory of the
// store was deleted.
type StoreRemoveResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}

// StoreRemove takes the store name out of config.yaml and deletes its
// directory. With keep it only takes the store out; a directory that is not
// there is not deleted either. The deletion is refused, with KindConflict
// "store.unpushed", when the tasks could not be got back: commits are not
// pushed, or there is no remote. The store lock is held from that check until
// the directory is gone.
func StoreRemove(ss *usecase.Session, name string, keep bool) (StoreRemoveResult, error) {
	c, _, err := ss.Config()
	if err != nil {
		return StoreRemoveResult{}, err
	}
	e, err := c.Select(name)
	if err != nil {
		if errors.Is(err, config.ErrNoStores) {
			return StoreRemoveResult{}, err
		}
		return StoreRemoveResult{}, usecase.NewErr(usecase.KindNotFound, "store.not_found", err, "name", name)
	}
	deleting := false
	var closeRepo func()
	if !keep {
		if _, serr := osfs.Stat(e.Path); !errors.Is(serr, fs.ErrNotExist) {
			release, closer, err := lockToDelete(ss, e)
			if err != nil {
				return StoreRemoveResult{}, err
			}
			defer release()
			closeRepo = closer
			deleting = true
		}
	}
	if err := ss.UpdateConfig(c, func(c *config.Config) error {
		if !usecase.StoreExists(c, name) { // another writer may have removed it since the lookup
			return usecase.NewErr(usecase.KindNotFound, "store.not_found", fmt.Errorf("no store is named %q", name), "name", name)
		}
		return c.RemoveStore(name)
	}); err != nil {
		return StoreRemoveResult{}, err
	}
	if deleting {
		// No git file of the directory may be open when it goes (Windows).
		closeRepo()
		ss.CloseMounted(e.Path)
		if err := osfs.RemoveAll(e.Path); err != nil {
			return StoreRemoveResult{}, err
		}
	}
	return StoreRemoveResult{Name: name, Path: e.Path, Deleted: deleting}, nil
}

// StoreDefault makes name the default store and returns the default; an empty
// name changes nothing and returns the current one ("" when there is none).
// config.yaml is not written when name is the default already. A name that
// is no store is KindNotFound, "store.not_found", with the names as
// candidates.
func StoreDefault(ss *usecase.Session, name string) (string, error) {
	c, _, err := ss.Config()
	if err != nil {
		return "", err
	}
	if name == "" || name == c.DefaultStore {
		return c.DefaultStore, nil
	}
	if err := ss.UpdateConfig(c, func(c *config.Config) error {
		if !usecase.StoreExists(c, name) { // also when another writer removed it since the read
			return noSuchStore(c, name)
		}
		return c.SetDefaultStore(name)
	}); err != nil {
		return "", err
	}
	return c.DefaultStore, nil
}

// noSuchStore is the error for a name that is no store of c: KindNotFound,
// "store.not_found", with the names as candidates.
func noSuchStore(c *config.Config, name string) error {
	names := make([]string, len(c.Stores))
	for i, s := range c.Stores {
		names[i] = s.Name
	}
	msg := fmt.Sprintf("no store is named %q", name)
	if len(names) > 0 {
		msg += " (stores: " + strings.Join(names, ", ") + ")"
	}
	return usecase.NewErr(usecase.KindNotFound, "store.not_found", errors.New(msg), "name", name, "candidates", names)
}

// StoreSet changes how the GUI shows the store name, as hh store set: its
// color (one of config.StoreColors; "" gives it back to the GUI to pick) and
// whether it is hidden. A nil argument is left as it is. config.yaml is not
// written when nothing changes. A color that is not one is KindInvalid,
// "store.bad_color"; a name that is no store is "store.not_found" as for
// StoreDefault. It returns the store as it is now.
func StoreSet(ss *usecase.Session, name string, color *string, hidden *bool) (StoreView, error) {
	if color != nil && *color != "" && !slices.Contains(config.StoreColors, *color) {
		return StoreView{}, usecase.NewErr(usecase.KindInvalid, "store.bad_color",
			fmt.Errorf("%q is not a store color (colors: %s)", *color, strings.Join(config.StoreColors, ", ")),
			"value", *color, "candidates", slices.Clone(config.StoreColors))
	}
	c, _, err := ss.Config()
	if err != nil {
		return StoreView{}, err
	}
	i := slices.IndexFunc(c.Stores, func(e config.StoreEntry) bool { return e.Name == name })
	if i < 0 {
		return StoreView{}, noSuchStore(c, name)
	}
	cur := c.Stores[i]
	if (color != nil && *color != cur.Color) || (hidden != nil && *hidden != cur.Hidden) {
		if err := ss.UpdateConfig(c, func(c *config.Config) error {
			if !usecase.StoreExists(c, name) { // another writer may have removed it since the read
				return noSuchStore(c, name)
			}
			return c.SetStoreLook(name, color, hidden)
		}); err != nil {
			return StoreView{}, err
		}
	}
	i = slices.IndexFunc(c.Stores, func(e config.StoreEntry) bool { return e.Name == name })
	return storeView(ss, c, c.Stores[i]), nil
}

// lockToDelete checks that deleting the directory of e loses nothing, and
// returns the function that lets go of the lock and the handle of the store,
// which the caller calls once the directory is gone.
func lockToDelete(ss *usecase.Session, e config.StoreEntry) (release, closeRepo func(), err error) {
	unpushed := func(err error) error {
		return usecase.NewErr(usecase.KindConflict, "store.unpushed", err, "name", e.Name)
	}
	if len(e.Remotes) == 0 {
		return nil, nil, unpushed(fmt.Errorf("store %s has no remote, so removing %s would lose its tasks for good; use --keep to take the store out of the config and leave the directory, or delete the directory yourself if you do not need it", e.Name, e.Path))
	}
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return nil, nil, unpushed(fmt.Errorf("store %s: %w; delete the directory by hand if you mean to, or use --keep to take the store out of the config and leave it", e.Name, err))
	}
	l, err := store.LockInOrder(s)
	if err != nil {
		_ = s.Repo().Close()
		return nil, nil, usecase.Classify(err)
	}
	release = func() {
		l.Release()
		_ = s.Repo().Close()
	}
	closeRepo = func() { _ = s.Repo().Close() }
	if err := checkPushed(e, s); err != nil {
		release()
		return nil, nil, err
	}
	return release, closeRepo, nil
}

// checkPushed refuses a store whose commits are not all on its origin, or
// that has changes that are not committed.
func checkPushed(e config.StoreEntry, s *store.Store) error {
	unpushed := func(err error) error {
		return usecase.NewErr(usecase.KindConflict, "store.unpushed", err, "name", e.Name)
	}
	n, err := usecase.Unpushed(s)
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
		return unpushed(fmt.Errorf("store %s has changes that are not committed (%s); commit or restore them, or use --keep to leave the directory, or delete it by hand if you mean to lose them", e.Name, strings.Join(paths, ", ")))
	}
	if n > 0 {
		return unpushed(fmt.Errorf("store %s has %s not pushed; run hedgehug sync --store %s first, or use --keep to leave the directory", e.Name, plural.Count(n, "commit"), e.Name))
	}
	return nil
}
