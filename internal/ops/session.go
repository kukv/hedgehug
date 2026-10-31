package ops

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/query"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// syncLockWait is how long a sync waits for another one of the store, such
// as the one in the background, to end.
const syncLockWait = 5 * time.Minute

// Session is what one caller (a CLI run, the GUI server) works through: the
// outside world it sees and the store it works on.
type Session struct {
	Getenv     func(string) string
	Now        func() time.Time
	StoreName  string                                    // --store; "" selects as config.Select does
	Warn       func(msg string)                          // warnings; the CLI prints "warning: "+msg to stderr
	AfterWrite func(s *store.Store, e config.StoreEntry) // after a commit; nil does nothing more

	// the writer of the store, read from config.yaml once per session
	writerDone bool
	writerAcct config.Account
	writerOK   bool
	writerErr  error
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
		return nil, config.StoreEntry{}, err
	}
	return c, e, nil
}

// OpenStore opens the store the session works on.
func (ss *Session) OpenStore() (*store.Store, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	return store.Open(e.Path)
}

// Open opens the store and its index.
func (ss *Session) Open() (*store.Store, *index.Index, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return nil, nil, err
	}
	ix, err := index.Open(s)
	if err != nil {
		return nil, nil, err
	}
	return s, ix, nil
}

// Writer returns the account that writes to the store the session works
// on; ok is false when the store has none. It reads the config once.
func (ss *Session) Writer() (acct config.Account, ok bool, err error) {
	if !ss.writerDone {
		ss.writerDone = true
		ss.writerAcct, ss.writerOK, ss.writerErr = ss.loadWriter()
	}
	return ss.writerAcct, ss.writerOK, ss.writerErr
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

// Update runs one operation as one commit. The author is the store's
// account, or the git config when the store has none. committed is false
// when the operation changed nothing.
func (ss *Session) Update(s *store.Store, op store.Op, fn func(*store.Tx) error) (committed bool, err error) {
	acct, ok, err := ss.Writer()
	if err != nil {
		return false, err
	}
	author, err := config.CommitAuthor(acct, ok, ss.Getenv)
	if err != nil {
		return false, err
	}
	op.Author = author
	if ok {
		op.Account = acct.ID()
	}
	beforeUpdate()
	committed, err = s.Update(op, fn)
	if err == nil && committed {
		ss.markWritten(s)
	}
	return committed, err
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
	opt := sync.Options{Remotes: e.Remotes, Author: author, Me: c.Me(), Now: ss.Now}
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

// Resolve finds the task arg names.
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
		return nil, classify(err)
	}
	e, _ := ix.Task(ulid)
	return e, nil
}

// ResolveULID is Resolve for the callers that need only the ULID.
func (ss *Session) ResolveULID(ix *index.Index) func(string) (model.ULID, error) {
	return func(s string) (model.ULID, error) {
		e, err := ss.Resolve(ix, s)
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
	r := s.Repo()
	head, err := r.Head()
	if err != nil {
		return 0, err
	}
	remote, err := r.RemoteHead("origin")
	if err != nil {
		return 0, err
	}
	es, err := r.Between(head, remote)
	return len(es), err
}
