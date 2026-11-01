package ops

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	gosync "sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/auth"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/mount"
	"github.com/kukv/hedgehug/internal/query"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// syncLockWait is how long a sync waits for another one of the store, such
// as the one in the background, to end.
const syncLockWait = 5 * time.Minute

// Session is what one caller works through: the outside world it sees and
// the store it works on. The CLI makes one per run and the GUI server one per
// request, so that the warnings and the writer it reads are those of the one
// caller. Operations never assign to its fields; what it caches is guarded
// by a mutex, so concurrent operations on one Session are safe.
//
// Every exported field is copied by ForStore.
type Session struct {
	Getenv     func(string) string
	Now        func() time.Time
	Loc        *time.Location                            // where a date without a zone is read; nil is time.Local
	StoreName  string                                    // --store; "" selects as config.Select does
	Warn       func(msg string)                          // warnings; the CLI prints "warning: "+msg to stderr
	AfterWrite func(s *store.Store, e config.StoreEntry) // after a commit; nil does nothing more
	// BeforeUpdate lets a test write to the store after an operation read its
	// index and before it writes, or write config.yaml after an operation
	// read it and before it updates it, as another process could.
	BeforeUpdate func()
	Keyring      auth.Keyring // nil is the OS keychain
	HTTPClient   *http.Client // nil is a client with a 30-second timeout
	// Run runs a command (gh) and gives its output; nil runs the command.
	Run func(name string, args ...string) (stdout, stderr []byte, err error)

	// the writer of the store, read from config.yaml once per session; an
	// error is not kept, so the next call reads again
	writerMu   gosync.Mutex
	writerDone bool
	writerAcct config.Account
	writerOK   bool

	// the tokens got, shared with the sessions ForStore makes
	tokMu  gosync.Mutex
	tokens *tokenCache

	// the stores mounted, shared with the sessions ForStore makes
	mountMu gosync.Mutex
	mounts  *mountCache
}

// mountCache is the set of mounted stores, opened once for all the sessions
// of one caller. A failure is not kept, so the next call opens again.
type mountCache struct {
	mu  gosync.Mutex
	set *mount.Set
	// the id prefixes that stores share, which qualified works out; sharedMu
	// is apart from mu, as Mount holds mu while it asks for them
	sharedMu   gosync.Mutex
	shared     func(prefix string) bool
	sharedDone bool
	// the stores that did not open and were warned of, once per caller
	warnedMu gosync.Mutex
	warned   map[string]bool
}

// ForStore is a session of its own on the store name, with the same outside
// world and hooks as ss. Not a copy of ss: that would carry the writer ss
// cached for another store.
func (ss *Session) ForStore(name string) *Session {
	return &Session{
		Getenv:       ss.Getenv,
		Now:          ss.Now,
		Loc:          ss.Loc,
		StoreName:    name,
		Warn:         ss.Warn,
		AfterWrite:   ss.AfterWrite,
		BeforeUpdate: ss.BeforeUpdate,
		Keyring:      ss.Keyring,
		HTTPClient:   ss.HTTPClient,
		Run:          ss.Run,
		tokens:       ss.tokenCache(),
		mounts:       ss.mountCache(),
	}
}

// mountCache is the cache of ss, made on first use.
func (ss *Session) mountCache() *mountCache {
	ss.mountMu.Lock()
	defer ss.mountMu.Unlock()
	if ss.mounts == nil {
		ss.mounts = &mountCache{}
	}
	return ss.mounts
}

// Mount is the stores config.yaml lists, opened once per caller. A store
// that does not open is in Skipped, and the first call warns of each.
func (ss *Session) Mount() (*mount.Set, error) {
	mc := ss.mountCache()
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.set != nil {
		return mc.set, nil
	}
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	set, err := mount.Open(c, mount.Options{Now: func() time.Time { return ss.Now() }, Warn: ss.warn, Qualified: ss.qualifiedNow()})
	if err != nil {
		return nil, err
	}
	for _, sk := range set.Skipped {
		ss.warnSkipped(sk)
	}
	mc.set = set
	return set, nil
}

// warnSkipped warns that a store did not open, once per caller however many
// times the stores are opened.
func (ss *Session) warnSkipped(sk mount.Skipped) {
	mc := ss.mountCache()
	mc.warnedMu.Lock()
	defer mc.warnedMu.Unlock()
	if mc.warned[sk.Name] {
		return
	}
	if mc.warned == nil {
		mc.warned = map[string]bool{}
	}
	mc.warned[sk.Name] = true
	ss.warn(openError(sk.Name, sk.Err).Error())
}

// dropMount forgets the stores mounted, for the next call to open them
// again. A write through a handle of its own (a sync) adds objects that a
// store opened before it does not see.
func (ss *Session) dropMount() {
	mc := ss.mountCache()
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.set = nil
	ss.forgetQualified()
}

// openError is the error of a store that does not open.
func openError(name string, err error) *Error {
	return newErr(KindInvalid, "store.open", fmt.Errorf("cannot open %s: %w", name, err), "name", name)
}

// location is where a date without a zone is read.
func (ss *Session) location() *time.Location {
	if ss.Loc != nil {
		return ss.Loc
	}
	return time.Local
}

func (ss *Session) warn(msg string) {
	if ss.Warn != nil {
		ss.Warn(msg)
	}
}

// Config loads config.yaml.
func (ss *Session) Config() (*config.Config, config.Paths, error) {
	p, err := config.Resolve(ss.Getenv)
	if err != nil {
		return nil, p, err
	}
	c, err := config.Load(p)
	return c, p, err
}

// Entry is the config and the store the session works on.
func (ss *Session) Entry() (*config.Config, config.StoreEntry, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, config.StoreEntry{}, err
	}
	e, err := c.Select(ss.StoreName)
	if err != nil {
		return nil, config.StoreEntry{}, selectError(ss.StoreName, err)
	}
	return c, e, nil
}

// selectError classifies what config.Select refuses: a name that is no
// store, or no name when there are several. KindInvalid, not KindUsage: the
// CLI exits 1 for both.
func selectError(name string, err error) error {
	switch {
	case errors.Is(err, config.ErrNoStores):
		return err
	case name != "":
		return newErr(KindNotFound, "store.not_found", err, "name", name)
	}
	return newErr(KindInvalid, "store.choose", err)
}

// OpenStore opens the store the session works on.
func (ss *Session) OpenStore() (*store.Store, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return nil, openError(e.Name, err)
	}
	return s, nil
}

// openStoreAt opens the store in dir with the clock of the session, so that
// what it writes has the time ss.Now tells.
func (ss *Session) openStoreAt(dir string) (*store.Store, error) {
	s, err := store.Open(dir)
	if err != nil {
		return nil, err
	}
	s.SetNow(ss.Now)
	return s, nil
}

// Open opens the store and its index.
func (ss *Session) Open() (*store.Store, *index.Index, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return nil, nil, err
	}
	ix, err := ss.openIndex(s)
	if err != nil {
		return nil, nil, err
	}
	return s, ix, nil
}

// openIndex opens the index of s, which knows the name of its store. The
// store and the index are of the operation, not the ones mount keeps: an
// index sets the id numbers of its store, so the two go together, and
// operations run concurrently on one session.
func (ss *Session) openIndex(s *store.Store) (*index.Index, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return index.Open(s)
	}
	return ss.indexOf(e.Name, s)
}

// indexOf opens the index of s, the store called name, that knows its
// name and shows ids as the other indexes do.
func (ss *Session) indexOf(name string, s *store.Store) (*index.Index, error) {
	ix, err := index.Open(s)
	if err != nil {
		return nil, err
	}
	ix.SetStoreName(name)
	ix.SetQualified(ss.qualified())
	return ix, nil
}

// qualified tells which id prefixes show with the store's name, because two
// or more stores of config.yaml use them (mount.SharedPrefixes). It is worked out once
// per caller, and again after a write, which may add a workspace or change a
// prefix. Stores that cannot be read have no prefix to count.
func (ss *Session) qualified() func(prefix string) bool {
	mc := ss.mountCache()
	mc.sharedMu.Lock()
	defer mc.sharedMu.Unlock()
	if !mc.sharedDone {
		// the config as it is now: a store may have been added or removed
		c, _, err := ss.Config()
		if err != nil {
			return nil // not kept, so the next call reads again
		}
		mc.shared, mc.sharedDone = mount.SharedPrefixes(c), true
	}
	return mc.shared
}

// qualifiedNow is qualified asked again at each use, for what is kept long:
// a mounted set must follow a write that changes which prefixes are shared.
func (ss *Session) qualifiedNow() func(prefix string) bool {
	return func(prefix string) bool {
		fn := ss.qualified()
		return fn != nil && fn(prefix)
	}
}

// forgetQualified drops what qualified worked out, for the next call to
// work it out again.
func (ss *Session) forgetQualified() {
	mc := ss.mountCache()
	mc.sharedMu.Lock()
	defer mc.sharedMu.Unlock()
	mc.shared, mc.sharedDone = nil, false
}

// Writer returns the account that writes to the store the session works
// on; ok is false when the store has none. It reads the config once; a
// failed read is not kept.
func (ss *Session) Writer() (acct config.Account, ok bool, err error) {
	ss.writerMu.Lock()
	defer ss.writerMu.Unlock()
	if !ss.writerDone {
		acct, ok, err = ss.loadWriter()
		if err != nil {
			return config.Account{}, false, err
		}
		ss.writerDone, ss.writerAcct, ss.writerOK = true, acct, ok
	}
	return ss.writerAcct, ss.writerOK, nil
}

func (ss *Session) loadWriter() (config.Account, bool, error) {
	c, e, err := ss.Entry()
	if err != nil {
		return config.Account{}, false, err
	}
	acct, ok := c.Writer(e)
	return acct, ok, nil
}

// AccountID is the ID of the writer's account, or "" when the store has none.
func (ss *Session) AccountID() (string, error) {
	acct, ok, err := ss.Writer()
	if err != nil || !ok {
		return "", err
	}
	return acct.ID(), nil
}

// Update is UpdateCommit for the callers that need only to know whether a
// commit was made.
func (ss *Session) Update(s *store.Store, op store.Op, fn func(*store.Tx) error) (committed bool, err error) {
	h, err := ss.UpdateCommit(s, op, fn)
	return !h.IsZero(), err
}

// UpdateCommit runs one operation as one commit and returns its hash, the
// zero hash when the operation changed nothing. The author is the store's
// account, or the git config when the store has none.
func (ss *Session) UpdateCommit(s *store.Store, op store.Op, fn func(*store.Tx) error) (plumbing.Hash, error) {
	// Operations refuse a read-only store when they open it (OpenForWrite);
	// this catches a write that did not come through there.
	if err := ss.checkWritable(s); err != nil {
		return plumbing.ZeroHash, err
	}
	acct, ok, err := ss.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	author, err := config.CommitAuthor(acct, ok, ss.Getenv)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	op.Author = author
	if ok {
		op.Account = acct.ID()
	}
	if ss.BeforeUpdate != nil {
		ss.BeforeUpdate()
	}
	h, err := s.UpdateCommit(op, fn)
	if err == nil && !h.IsZero() {
		ss.markWritten(s)
		ss.forgetQualified()
	}
	return h, err
}

// commitHex is the hex form of a commit hash, "" for the zero hash.
func commitHex(h plumbing.Hash) string {
	if h.IsZero() {
		return ""
	}
	return h.String()
}

// markWritten notes that s has a commit to push and hands the write to
// AfterWrite. The commit is made, so a failure here is only a warning.
func (ss *Session) markWritten(s *store.Store) {
	if err := sync.MarkPending(s, ss.Now()); err != nil {
		ss.warn("the write is not marked for sync: " + err.Error())
		return
	}
	if ss.AfterWrite == nil {
		return
	}
	_, e, err := ss.Entry()
	if err != nil {
		return
	}
	ss.AfterWrite(s, e)
}

// SyncOptions is what sync.Run needs for the store e, which need not be in c
// yet: the account that writes, the accounts that are me, and the other
// stores whose id prefixes it checks.
func (ss *Session) SyncOptions(c *config.Config, e config.StoreEntry) (sync.Options, error) {
	acct, ok := c.Writer(e)
	author, err := config.CommitAuthor(acct, ok, ss.Getenv)
	if err != nil {
		return sync.Options{}, err
	}
	opt := sync.Options{Name: e.Name, Remotes: e.Remotes, Author: author, Me: c.Me(), Now: ss.Now, Conn: ss.Conn(c, e), GitHub: ss.githubRepoOf(c, e)}
	if ok {
		opt.Account = acct.ID()
	}
	for _, o := range c.Stores {
		if o.Name != e.Name {
			opt.Others = append(opt.Others, sync.OtherStore{Name: o.Name, Path: o.Path})
		}
	}
	return opt, nil
}

// QueryOptions is what the queries on the store s need besides its index: the
// accounts @me stands for, and the tasks with an unread overwritten item in
// the inbox.
func (ss *Session) QueryOptions(s *store.Store) (query.Options, error) {
	c, _, err := ss.Config()
	if err != nil {
		return query.Options{}, err
	}
	items, err := inbox.Open(s).List(false)
	if err != nil {
		return query.Options{}, err
	}
	opt := query.Options{Me: c.Me(), Overwritten: map[model.ULID]bool{}}
	for _, it := range items {
		if it.Kind == inbox.Overwritten && !it.Task.IsZero() {
			opt.Overwritten[it.Task] = true
		}
	}
	return opt, nil
}

// ambiguousError is index.AmbiguousError with the candidates spelled out.
type ambiguousError struct {
	input string
	noun  string   // what the candidates are, "tasks" when empty
	lines []string // "<id> (<ULID>) <title>" per candidate
}

func (e *ambiguousError) Error() string {
	noun := e.noun
	if noun == "" {
		noun = "tasks"
	}
	return fmt.Sprintf("%q matches %d %s:\n  %s", e.input, len(e.lines), noun, strings.Join(e.lines, "\n  "))
}

// Resolve finds the task arg names in the store of ix.
func (ss *Session) Resolve(ix *index.Index, arg string) (*index.Entry, error) {
	ulid, err := ix.Resolve(arg)
	var ae *index.AmbiguousError
	if errors.As(err, &ae) {
		lines := make([]string, len(ae.Candidates))
		for i, c := range ae.Candidates {
			lines[i] = RefName(ix, c)
		}
		return nil, newErr(KindAmbiguous, "task.ambiguous", &ambiguousError{input: arg, lines: lines}, "input", arg)
	}
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, newErr(KindNotFound, "task.not_found", err, "input", arg)
		}
		return nil, classify(err)
	}
	e, _ := ix.Task(ulid)
	return e, nil
}

// ResolveULID is ResolveRef for the callers that need only the ULID.
func (ss *Session) ResolveULID(ix *index.Index) func(string) (model.ULID, error) {
	return func(s string) (model.ULID, error) {
		e, err := ss.ResolveRef(ix, s)
		if err != nil {
			return model.ULID{}, err
		}
		return e.ULID, nil
	}
}

// RunSync syncs s under .hedgehug/local/sync.lock, and keeps in sync.json
// how it went. A write that was pending when the sync began has been pushed
// by it; one made later has not. reported says that the one who ran it is
// told of a failure, so the next command does not tell of it again.
func (ss *Session) RunSync(s *store.Store, opt sync.Options, reported bool) (res sync.Result, err error) {
	// The sync writes objects through the handle s; the stores mounted are
	// opened again after it, so that they see them.
	defer ss.dropMount()
	err = s.WithLocalLock("sync.lock", syncLockWait, func() error {
		started := ss.Now()
		if _, err := sync.UpdateState(s, func(st *sync.State) { st.LastAttempt = started }); err != nil {
			return err
		}
		var runErr error
		res, runErr = sync.Run(s, opt)
		_, stateErr := sync.UpdateState(s, func(st *sync.State) {
			if runErr != nil {
				st.LastError = runErr.Error()
				st.ErrorReported = reported
				return
			}
			st.LastError, st.ErrorReported = "", false
			st.LastSuccess = ss.Now()
			// A write made while the sync ran finds pending_since set and
			// leaves it; it stays pending while HEAD has commits origin lacks.
			// Counted under the state lock, a commit after the count marks
			// pending again.
			if n, err := Unpushed(s); st.PendingSince.Before(started) && err == nil && n == 0 {
				st.PendingSince = time.Time{}
			}
		})
		return errors.Join(runErr, stateErr)
	})
	return res, err
}

// Unpushed counts the commits of s that origin did not have as of the last
// fetch or push. A store that never pushed has all of them.
func Unpushed(s *store.Store) (int, error) {
	return sync.Unpushed(s.Repo())
}
