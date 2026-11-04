package usecase

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	gosync "sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
	"github.com/kukv/hedgehug/internal/app/storage/query"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// SyncLockWait is how long a sync waits for another one of the store, such
// as the one in the background, to end.
const SyncLockWait = 5 * time.Minute

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
	Loc        *time.Location         // where a date without a zone is read; nil is time.Local
	StoreName  string                 // --store; "" selects as config.Select does
	Warn       func(msg string)       // warnings; the CLI prints "warning: "+msg to stderr
	AfterWrite func(storeName string) // after a commit; nil does nothing more
	// BackgroundSync starts a background sync after a write to a store with a
	// remote (the CLI); the GUI server syncs through its own loop.
	// ForStore copies it.
	BackgroundSync bool
	// BeforeUpdate lets a test write to the store after an operation read its
	// index and before it writes, or write config.yaml after an operation
	// read it and before it updates it, as another process could.
	BeforeUpdate func()
	// BeforeSwap is told the path of a store before doctor --fix swaps the
	// directory of a rebuild in for it (after a force push): whoever holds
	// that store open, such as the GUI server's index, lets go of it.
	// ForStore copies it. nil does nothing.
	BeforeSwap func(path string)
	Keyring    Keyring      // nil is the OS keychain
	HTTPClient *http.Client // nil is a client with a 30-second timeout
	// Progress is told how far the history index of a store has come while it
	// is built on the spot: done of total commits. ForStore copies it; nil
	// does nothing.
	Progress func(store string, done, total int)
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
	// what the session opened and has not let go of (Close): the stores
	// OpenStoreAt made and the sets listSet made
	openedMu gosync.Mutex
	opened   []func()
	// the id prefixes that stores share, which qualified works out; sharedMu
	// is apart from mu, as Mount holds mu while it asks for them
	sharedMu   gosync.Mutex
	shared     func(prefix string) bool
	sharedDone bool
	// the stores that did not open and were warned of, once per caller
	warnedMu gosync.Mutex
	warned   map[string]bool
	muted    map[string]int // callers that tell of the store themselves, by store
}

// ForStore is a session of its own on the store name, with the same outside
// world and hooks as ss. Not a copy of ss: that would carry the writer ss
// cached for another store.
func (ss *Session) ForStore(name string) *Session {
	return &Session{
		Getenv:         ss.Getenv,
		Now:            ss.Now,
		Loc:            ss.Loc,
		StoreName:      name,
		Warn:           ss.Warn,
		AfterWrite:     ss.AfterWrite,
		BackgroundSync: ss.BackgroundSync,
		BeforeUpdate:   ss.BeforeUpdate,
		BeforeSwap:     ss.BeforeSwap,
		Keyring:        ss.Keyring,
		HTTPClient:     ss.HTTPClient,
		Run:            ss.Run,
		Progress:       ss.Progress,
		tokens:         ss.tokenCache(),
		mounts:         ss.mountCache(),
	}
}

// ForgetMounts lets go of the stores the session mounted, as a new run of
// the command starts without them.
func (ss *Session) ForgetMounts() { ss.mounts = nil }

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
// that does not open is in Skipped, and the first call warns of each (not of
// the others when --store names one store).
func (ss *Session) Mount() (*mount.Set, error) {
	set, toWarn, err := ss.mountOnce()
	// not under mc.mu: Warn is the caller's and may call back
	for _, sk := range toWarn {
		ss.WarnSkipped(sk)
	}
	return set, err
}

// mountOnce is the cached set, opened if need be; toWarn are the stores of
// a set opened now that the caller tells of.
func (ss *Session) mountOnce() (set *mount.Set, toWarn []mount.Skipped, err error) {
	mc := ss.mountCache()
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.set != nil {
		return mc.set, nil, nil
	}
	c, _, err := ss.Config()
	if err != nil {
		return nil, nil, err
	}
	set, toWarn, _ = ss.OpenSet(c, false)
	mc.set = set
	return set, toWarn, nil
}

// OpenSet opens the stores of c as this session wants them: its clock, the
// id prefixes it counts itself, and what Resolve finds not to open given to
// WarnSkipped. A store that does not open is left out of the set. toWarn are
// those the caller is to tell of with WarnSkipped, once it holds no lock: all
// of them, except that with --store naming a store the command is about
// that store only. With strict the store --store names, and the only store
// of c, are an error instead (what a listing does).
func (ss *Session) OpenSet(c *config.Config, strict bool) (set *mount.Set, toWarn []mount.Skipped, err error) {
	set = mount.Open(c, mount.Options{Now: func() time.Time { return ss.Now() }, Unopened: ss.WarnSkipped, Qualified: ss.qualifiedNow()})
	for _, sk := range set.Skipped {
		switch {
		case strict && (ss.StoreName == sk.Name || len(c.Stores) == 1):
			set.Close() // the caller gets no set to close
			return nil, nil, OpenError(sk.Name, sk.Err)
		case ss.StoreName != "" && ss.StoreName != sk.Name:
			// not what the command is about
		default:
			toWarn = append(toWarn, sk)
		}
	}
	for _, u := range set.Unfinished {
		if ss.StoreName == "" || ss.StoreName == u.Name {
			toWarn = append(toWarn, u)
		}
	}
	return set, toWarn, nil
}

// WarnSkipped warns that a store did not open: the stores found not to open
// when they are mounted, and an index that does not open while an id is
// looked up in every store. It is said once per caller however many times
// the stores are opened, and not while MuteOpenWarnings holds the store. (The
// other warn-once of the package is the context.json one, which AddHere
// leaves out for a call that carries an answer; that is about a file, not
// a store.)
func (ss *Session) WarnSkipped(sk mount.Skipped) {
	var left *store.LeftoverError
	if errors.As(sk.Err, &left) {
		ss.WarnOnce(sk.Name, left.Error())
		return
	}
	ss.WarnOnce(sk.Name, OpenError(sk.Name, sk.Err).Error())
}

// WarnOnce says msg about the store name, once per caller and not while
// MuteOpenWarnings holds the store.
func (ss *Session) WarnOnce(name, msg string) {
	mc := ss.mountCache()
	mc.warnedMu.Lock()
	if mc.warned[name] || mc.muted[name] > 0 {
		mc.warnedMu.Unlock()
		return
	}
	if mc.warned == nil {
		mc.warned = map[string]bool{}
	}
	mc.warned[name] = true
	mc.warnedMu.Unlock()
	ss.AddWarning(msg)
}

// MuteOpenWarnings makes the mount keep quiet about the stores names that do
// not open, for a caller that tells of them in its own way. The returned
// function ends it: once the last caller that muted a store has ended, the
// next mount, opened again for that, warns of it. Callers may overlap.
func (ss *Session) MuteOpenWarnings(names []string) (unmute func()) {
	mc := ss.mountCache()
	mc.warnedMu.Lock()
	if mc.muted == nil {
		mc.muted = map[string]int{}
	}
	for _, n := range names {
		mc.muted[n]++
	}
	mc.warnedMu.Unlock()
	return func() {
		if len(names) == 0 {
			return
		}
		mc.warnedMu.Lock()
		for _, n := range names {
			if mc.muted[n]--; mc.muted[n] <= 0 {
				delete(mc.muted, n)
			}
		}
		mc.warnedMu.Unlock()
		ss.DropMount()
	}
}

// Keep notes how to let go of something the session opened, for Close.
func (ss *Session) Keep(closeFn func()) {
	mc := ss.mountCache()
	mc.openedMu.Lock()
	defer mc.openedMu.Unlock()
	mc.opened = append(mc.opened, closeFn)
}

// Close lets go of the repository files of every store the session opened
// and of the stores it mounted. The GUI server calls it at the end of a
// request: a store that is not closed keeps its pack files open until it is
// collected, and Windows cannot rename or remove them. It may be called any
// number of times, and the session may be used after: what it opens again is
// noted again. Not to be called while an operation of the session runs.
func (ss *Session) Close() {
	mc := ss.mountCache()
	mc.openedMu.Lock()
	opened := mc.opened
	mc.opened = nil
	mc.openedMu.Unlock()
	for _, closeFn := range opened {
		closeFn()
	}
	ss.DropMount()
}

// CloseMounted lets go of the repository files of the mounted store whose
// directory is dir, so that a rename of the directory does not meet them
// open (Windows).
func (ss *Session) CloseMounted(dir string) {
	mc := ss.mountCache()
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.set == nil {
		return
	}
	for _, m := range mc.set.Stores {
		if !osfs.SamePath(m.Entry.Path, dir) {
			continue
		}
		_ = m.CloseRepo()
	}
}

// DropMount forgets the stores mounted, for the next call to open them
// again, after it lets go of their files. A write through a handle of its
// own (a sync) adds objects that a store opened before it does not see.
func (ss *Session) DropMount() {
	mc := ss.mountCache()
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.set != nil {
		mc.set.Close()
	}
	mc.set = nil
	ss.forgetQualified()
}

// OpenError is the error of a store that does not open.
func OpenError(name string, err error) *Error {
	return NewErr(KindInvalid, "store.open", fmt.Errorf("cannot open %s: %w", name, err), "name", name)
}

// Location is where a date without a zone is read.
func (ss *Session) Location() *time.Location {
	if ss.Loc != nil {
		return ss.Loc
	}
	return time.Local
}

func (ss *Session) AddWarning(msg string) {
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
		return nil, config.StoreEntry{}, SelectError(ss.StoreName, err)
	}
	return c, e, nil
}

// SelectError classifies what config.Select refuses: a name that is no
// store, or no name when there are several. KindInvalid, not KindUsage: the
// CLI exits 1 for both.
func SelectError(name string, err error) error {
	switch {
	case errors.Is(err, config.ErrNoStores):
		return err
	case name != "":
		return NewErr(KindNotFound, "store.not_found", err, "name", name)
	}
	return NewErr(KindInvalid, "store.choose", err)
}

// OpenStore opens the store the session works on.
func (ss *Session) OpenStore() (*store.Store, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	s, err := ss.OpenStoreAt(e.Path)
	left, lerr := store.FindLeftover(e.Path)
	unfinished := lerr == nil && !left.None()
	if err != nil {
		if unfinished {
			err = &store.LeftoverError{Name: e.Name}
		}
		return nil, OpenError(e.Name, err)
	}
	if unfinished {
		ss.WarnOnce(e.Name, store.LeftoverMessage(e.Name))
	}
	return s, nil
}

// OpenStoreAt opens the store in dir with the clock of the session, so that
// what it writes has the time ss.Now tells.
func (ss *Session) OpenStoreAt(dir string) (*store.Store, error) {
	s, err := store.Open(dir)
	if err != nil {
		return nil, err
	}
	s.SetNow(ss.Now)
	ss.Keep(func() { _ = s.Repo().Close() })
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
	return ss.IndexOf(e.Name, s)
}

// IndexOf opens the index of s, the store called name, that knows its
// name and shows ids as the other indexes do.
func (ss *Session) IndexOf(name string, s *store.Store) (*index.Index, error) {
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
// account.
func (ss *Session) UpdateCommit(s *store.Store, op store.Op, fn func(*store.Tx) error) (plumbing.Hash, error) {
	// Operations refuse a read-only store when they open it (OpenForWrite);
	// this catches a write that did not come through there.
	if err := ss.CheckWritable(s); err != nil {
		return plumbing.ZeroHash, err
	}
	acct, ok, err := ss.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if !ok {
		return plumbing.ZeroHash, ErrNoAccounts
	}
	op.Author = config.CommitAuthor(acct)
	op.Account = acct.ID()
	if ss.BeforeUpdate != nil {
		ss.BeforeUpdate()
	}
	h, err := s.UpdateCommit(op, fn)
	if err == nil && !h.IsZero() {
		ss.MarkWritten(s)
		ss.forgetQualified()
	}
	return h, err
}

// CommitHex is the hex form of a commit hash, "" for the zero hash.
func CommitHex(h plumbing.Hash) string {
	if h.IsZero() {
		return ""
	}
	return h.String()
}

// MarkWritten notes that s has a commit to push, starts the background sync
// when the session does that and the store has a remote, and hands the write
// to AfterWrite. The commit is made, so a failure here is only a warning.
func (ss *Session) MarkWritten(s *store.Store) {
	if err := replicate.MarkPending(s, ss.Now()); err != nil {
		ss.AddWarning("the write is not marked for sync: " + err.Error())
		return
	}
	_, e, err := ss.Entry()
	if err != nil {
		return
	}
	if ss.BackgroundSync && len(e.Remotes) > 0 {
		if err := replicate.Start(s, e.Name, ss.Getenv); err != nil {
			ss.AddWarning("the background sync did not start: " + err.Error())
		}
	}
	if ss.AfterWrite != nil {
		ss.AfterWrite(e.Name)
	}
}

// AccountCount is the number of accounts config.yaml lists.
func (ss *Session) AccountCount() (int, error) {
	c, _, err := ss.Config()
	if err != nil {
		return 0, err
	}
	return len(c.Accounts), nil
}

// DataDir is the directory hedgehug keeps its data in.
func (ss *Session) DataDir() (string, error) {
	p, err := config.Resolve(ss.Getenv)
	if err != nil {
		return "", err
	}
	return p.DataDir, nil
}

// StoreRef is the name and path of a store.
type StoreRef struct{ Name, Path string }

// CurrentStore is the store the session works on.
func (ss *Session) CurrentStore() (StoreRef, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return StoreRef{}, err
	}
	return StoreRef{Name: e.Name, Path: e.Path}, nil
}

// TaskULID is the ULID of the task that id names.
func (ss *Session) TaskULID(id string) (model.ULID, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return model.ULID{}, err
	}
	e, err := ss.Resolve(ix, id)
	if err != nil {
		return model.ULID{}, err
	}
	return e.ULID, nil
}

// SyncIntervals is sync.interval and sync.push_interval of config.yaml.
func (ss *Session) SyncIntervals() (SyncIntervals, error) {
	c, _, err := ss.Config()
	if err != nil {
		return SyncIntervals{}, err
	}
	return SyncIntervals{Interval: c.Sync.Interval, PushInterval: c.Sync.PushInterval}, nil
}

// SyncOptions is what replicate.Run needs for the store e, which need not be in c
// yet: the account that writes, the accounts that are me, and the other
// stores whose id prefixes it checks.
func (ss *Session) SyncOptions(c *config.Config, e config.StoreEntry) (replicate.Options, error) {
	acct, ok := c.Writer(e)
	if !ok {
		return replicate.Options{}, ErrNoAccounts
	}
	opt := replicate.Options{Name: e.Name, Remotes: e.Remotes, Author: config.CommitAuthor(acct), Account: acct.ID(), Me: c.Me(), Now: ss.Now, Conn: ss.Conn(c, e), GitHub: ss.githubRepoOf(c, e)}
	for _, o := range c.Stores {
		if o.Name != e.Name {
			opt.Others = append(opt.Others, replicate.OtherStore{Name: o.Name, Path: o.Path})
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
		return nil, NewErr(KindAmbiguous, "task.ambiguous", &ambiguousError{input: arg, lines: lines}, "input", arg)
	}
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if moved := ss.MovedFor(ix, arg); moved != nil {
				return nil, moved
			}
			return nil, NewErr(KindNotFound, "task.not_found", err, "input", arg)
		}
		return nil, Classify(err)
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
// told of a failure, so the next command does not tell of it again. A
// history of origin that differs from the store's stops it (ErrForcePush);
// hedgehug doctor --fix clones the store again.
func (ss *Session) RunSync(s *store.Store, opt replicate.Options, reported bool) (res replicate.Result, err error) {
	// The sync writes objects through the handle s; the stores mounted are
	// opened again after it, so that they see them.
	defer ss.DropMount()
	err = s.WithLocalLock("sync.lock", SyncLockWait, func() error {
		started := ss.Now()
		if _, err := replicate.UpdateState(s, func(st *replicate.State) { st.LastAttempt = started }); err != nil {
			return err
		}
		var runErr error
		res, runErr = replicate.Run(s, opt)
		_, stateErr := replicate.UpdateState(s, func(st *replicate.State) {
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
	return replicate.Unpushed(s.Repo())
}

// ForgetWriter forgets the writer the session cached, so that it is read from
// config.yaml again after a test changed the accounts.
func (ss *Session) ForgetWriter() {
	ss.writerMu.Lock()
	defer ss.writerMu.Unlock()
	ss.writerDone = false
}
