// Package watch looks at the stores on disk and tells a Sink what changed:
// the commits of each store, its schema and views, the sync state of each
// store, the items not read, and config.yaml. The GUI server runs it every
// few hundred milliseconds and publishes what it tells.
package watch

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// historyRounds is how many times a build of the history index goes on when
// HEAD moved meanwhile; the next HEAD move starts another build.
const historyRounds = 3

// Change is what tasks.changed tells: the tasks whose file or comments
// changed, and those whose file is gone (purged), in the commit Commit (HEAD,
// or one of the commits since the last look). Op and Author are the
// Hedgehug-Op and Hedgehug-Author of the commit; empty for a commit made by
// hand or when unknown. ULIDs and Removed are never nil.
type Change struct {
	Store   string
	ULIDs   []string
	Removed []string
	Commit  string
	Op      string
	Author  string
}

// StoreState is the HEAD and the sync state of one store.
type StoreState struct {
	Store string
	Head  string
	Sync  usecase.SyncStatusView
}

// Sink is where what a look finds goes, in the order the look finds it. No
// method may call the Watcher back: Unread is called with the Watcher's mu
// held.
type Sink interface {
	StoresChanged()
	TasksChanged(Change)
	SchemaChanged(store string)
	ViewsChanged(store string)
	SyncState(usecase.SyncStatusView)
	Unread(count int)
}

// Options is what a Watcher needs from outside.
type Options struct {
	Session  func(storeName string) *usecase.Session // a session of its own for each look
	Schedule usecase.SyncSchedule                    // the loop that syncs; nil when none runs
	Sink     Sink
	Unread   func(*usecase.Session) (int, error) // how many items are not read, in every store
	Logf     func(format string, args ...any)
}

// configSnapshot is what of config.yaml decides what the GUI shows of the
// stores: each store (name, path, remotes, account, color, visibility), the
// accounts and the default store. A look that finds it different tells the
// clients with stores.changed.
type configSnapshot struct {
	Stores   []config.StoreEntry
	Accounts []config.Account
	Default  string
}

// Watcher reads the HEAD and the sync state of each store on every Poll. It
// looks at files, which no fake clock moves.
type Watcher struct {
	session  func(storeName string) *usecase.Session
	schedule usecase.SyncSchedule
	sink     Sink
	unreadOf func(*usecase.Session) (int, error)
	logf     func(format string, args ...any)

	// openHistory builds the history index of a store; history.Open. A seam
	// for the tests.
	openHistory func(*store.Store, history.Options) (*history.History, error)

	pollMu sync.Mutex      // one poll at a time; held while reading, not w.mu
	cfg    *configSnapshot // config.yaml as the last look read it; nil before the first look (pollMu)

	mu       sync.Mutex // the state below
	stores   map[string]*polled
	released map[string]bool // stores rebuilt, to tell the clients once
	restored map[string]bool // stores let go for a removal that failed; opened again without a word
	unread   int
	looked   bool   // a poll has run
	scanned  bool   // a poll has read the stores to the end (set under mu)
	lastFail string // the last error of unreadOf that was logged
}

// New makes a Watcher that has looked at nothing.
func New(o Options) *Watcher {
	return &Watcher{
		session: o.Session, schedule: o.Schedule, sink: o.Sink, unreadOf: o.Unread, logf: o.Logf,
		stores: map[string]*polled{}, openHistory: history.Open,
	}
}

// polled is what the watcher knows of a store.
type polled struct {
	path  string
	s     *store.Store
	ix    *index.Index
	head  plumbing.Hash
	sync  usecase.SyncStatusView
	stamp string

	// histBusy is true while a goroutine brings the history index of the
	// store to HEAD; histDone is that goroutine's. Whoever lets go of the
	// store (its directory may be rebuilt or removed) waits for histDone.
	histBusy   atomic.Bool
	histDone   sync.WaitGroup
	histCancel chan struct{} // closed to stop the build
	cancelOnce sync.Once
}

func newPolled(path string, s *store.Store) *polled {
	return &polled{path: path, s: s, histCancel: make(chan struct{})}
}

// stopHistory cancels the build of the history index of st and waits for it
// to end.
func (st *polled) stopHistory() {
	st.cancelOnce.Do(func() { close(st.histCancel) })
	st.histDone.Wait()
}

// Release lets go of the store at path, whose directory is about to be
// rebuilt after a force push (D26): its index and its
// repository files are closed, and the next poll opens the store again and
// tells the clients that its tasks changed. It waits for a poll that is
// reading, and cancels the build of the history index of the store and waits
// for it to end.
func (w *Watcher) Release(path string) {
	w.pollMu.Lock()
	defer w.pollMu.Unlock()
	w.releaseLocked(path)
}

// releaseLocked is Release with pollMu held. It returns the names of the
// stores it let go of.
func (w *Watcher) releaseLocked(path string) []string {
	var gone []*polled
	var names []string
	w.mu.Lock()
	for name, st := range w.stores {
		if !osfs.SamePath(st.path, path) {
			continue
		}
		gone = append(gone, st)
		names = append(names, name)
		delete(w.stores, name)
		if w.released == nil {
			w.released = map[string]bool{}
		}
		w.released[name] = true
		delete(w.restored, name)
	}
	w.mu.Unlock()
	// The build of the history index reads the files under the directory
	// through a handle of its own, so the directory must not be renamed or
	// removed until it is over; the wait is made without w.mu, so hello does
	// not wait.
	for _, st := range gone {
		st.stopHistory()
		_ = st.s.Repo().Close()
	}
	return names
}

// ReleaseFor lets go of the store at path as Release does, and runs fn (the
// removal of the store: its directory goes) before any poll can look again,
// so that no poll opens the store again while its config entry is still
// there. When fn succeeds the store is gone for good, so it is no longer
// marked as rebuilt (a store of that name that comes later is
// new). When fn fails the store is still there: the next poll opens it again
// without telling the clients anything (it is the same store, not a rebuilt
// one). fn must not wait for a poll.
func (w *Watcher) ReleaseFor(path string, fn func() error) error {
	w.pollMu.Lock()
	defer w.pollMu.Unlock()
	names := w.releaseLocked(path)
	err := fn()
	w.mu.Lock()
	for _, n := range names {
		delete(w.released, n)
		if err != nil {
			if w.restored == nil {
				w.restored = map[string]bool{}
			}
			w.restored[n] = true
		}
	}
	w.mu.Unlock()
	return err
}

// Poll reads every store once. With publish false it only records (for
// snapshot before Run has looked). The first sight of a store only records,
// as the HEAD does. The reads that take long (the sync state, the inbox)
// are made without w.mu, so snapshot does not wait for them.
func (w *Watcher) Poll(publish bool) {
	w.pollMu.Lock()
	defer w.pollMu.Unlock()
	w.pollLocked(publish)
}

// pollLocked is poll with pollMu held.
func (w *Watcher) pollLocked(publish bool) {
	defer func() {
		w.mu.Lock()
		w.looked = true
		w.mu.Unlock()
	}()
	ss := w.session("")
	defer ss.Close()
	c, _, err := ss.Config()
	if err != nil {
		return
	}
	snap := &configSnapshot{Stores: c.Stores, Accounts: c.Accounts, Default: c.DefaultStore}
	if publish && w.cfg != nil && !reflect.DeepEqual(w.cfg, snap) {
		w.sink.StoresChanged()
	}
	w.cfg = snap
	targets, err := ss.SyncTargets(c, true)
	if errors.Is(err, config.ErrNoStores) {
		// No store is a look to the end too: the one that comes after it (the first one,
		// made from Onboarding) is a store that appeared after the first look.
		targets, err = nil, nil
	}
	if err != nil {
		return
	}
	statuses := map[string]usecase.SyncStatusView{}
	if sts, err := usecase.SyncStatus(ss); err == nil {
		for _, st := range sts {
			statuses[st.Store] = usecase.NewSyncStatusView(st, w.schedule, usecase.SyncIntervals{Interval: c.Sync.Interval, PushInterval: c.Sync.PushInterval})
		}
	}
	// Only poll writes w.stores and the fields of a polled, and polls run
	// one at a time (pollMu), so it reads them without w.mu; what snapshot
	// reads is written under w.mu, all together at the end of the loop, so
	// that the loop (index, diffs) does not make hello wait.
	type update struct {
		name  string
		st    *polled
		head  plumbing.Hash
		sync  usecase.SyncStatusView
		fresh bool
	}
	var updates []update
	stamps := map[string]string{} // taken before the inbox is read
	seen := map[string]bool{}
	for _, e := range targets {
		seen[e.Name] = true
		st := w.stores[e.Name]
		fresh := false
		announced := false // the clients were told to read the store again
		if st == nil || !osfs.SamePath(st.path, e.Path) {
			// This session is not closed: the store it opens is kept in
			// w.stores, and closed when it is dropped.
			s, err := w.session(e.Name).OpenStore()
			if err != nil {
				continue
			}
			first, err := s.Repo().Head()
			if err != nil {
				_ = s.Repo().Close()
				continue
			}
			if st != nil {
				st.stopHistory() // the store of the old path is let go
				_ = st.s.Repo().Close()
			}
			st = newPolled(e.Path, s)
			fresh = true
			w.mu.Lock()
			rebuilt := w.released[e.Name]
			delete(w.released, e.Name)
			restored := w.restored[e.Name]
			delete(w.restored, e.Name)
			scanned := w.scanned
			w.mu.Unlock()
			if (rebuilt || scanned) && !restored && publish {
				// The store was made again, or it appeared after the first look: the
				// commits made before this look are not known (a page that made a
				// task in a store just created would never be told), so the clients
				// read it again.
				w.sink.TasksChanged(Change{Store: e.Name, ULIDs: []string{}, Removed: []string{}, Commit: first.String()})
				announced = true
			}
		}
		head, err := st.s.Repo().Head()
		if err != nil {
			if fresh { // not kept in w.stores
				st.stopHistory()
				_ = st.s.Repo().Close()
			}
			continue
		}
		stamp := inbox.Open(st.s).Stamp()
		if head != st.head {
			if !st.head.IsZero() && publish {
				w.headMoved(e.Name, st, head)
			}
			w.refresh(st)
			stamps[e.Name] = stamp
		}
		u := update{name: e.Name, st: st, head: head, sync: st.sync, fresh: fresh}
		if v, ok := statuses[e.Name]; ok && !reflect.DeepEqual(v, st.sync) {
			u.sync = v
			if publish && (!fresh || announced) {
				w.sink.SyncState(v)
			}
		}
		updates = append(updates, u)
		if stamp != st.stamp {
			stamps[e.Name] = stamp
		}
	}
	var dropped []*polled
	w.mu.Lock()
	for _, u := range updates {
		u.st.head, u.st.sync = u.head, u.sync
		w.stores[u.name] = u.st
	}
	w.scanned = true
	for name, st := range w.stores {
		if !seen[name] {
			dropped = append(dropped, st)
			delete(w.stores, name)
		}
	}
	w.mu.Unlock()
	// A store no longer targeted may be removed from the disk next.
	for _, st := range dropped {
		st.stopHistory()
		_ = st.s.Repo().Close()
	}
	if len(stamps) == 0 {
		return
	}
	n, err := w.unreadOf(ss)
	w.mu.Lock()
	defer w.mu.Unlock()
	// The stamps are those from before unreadOf, so a file another process
	// wrote meanwhile is seen by the next poll. A failure is not tried again
	// until a file or HEAD changes.
	for name, stamp := range stamps {
		if st := w.stores[name]; st != nil {
			st.stamp = stamp
		}
	}
	if err != nil {
		if msg := err.Error(); msg != w.lastFail {
			w.lastFail = msg
			w.logf("inbox: %s", msg)
		}
		return
	}
	w.lastFail = ""
	if n != w.unread {
		w.unread = n
		if publish {
			w.sink.Unread(w.unread)
		}
	}
}

// splitMax is the most commits of one look that headMoved tells one by one.
const splitMax = 50

// headMoved publishes the changes from st.head to head: commit by commit when
// head follows st.head by first parents (a page knows its own commits, and
// those of others in between are not shown as its changes; a sync is one
// merge commit), else as one change.
func (w *Watcher) headMoved(name string, st *polled, head plumbing.Hash) {
	r := st.s.Repo()
	if log, err := r.LogFrom(head, splitMax); err == nil {
		for i, e := range log {
			if len(e.Parents) == 0 || e.Parents[0] != st.head {
				continue
			}
			for j := i; j >= 0; j-- {
				from := st.head
				if j < i {
					from = log[j+1].Hash
				}
				w.publishDiff(name, st, from, log[j])
			}
			return
		}
	}
	e := gitrepo.LogEntry{Hash: head}
	if log, err := r.LogFrom(head, 1); err == nil && len(log) == 1 {
		e = log[0]
	}
	w.publishDiff(name, st, st.head, e)
}

// publishDiff publishes the changes from the commit from to the commit e.
func (w *Watcher) publishDiff(name string, st *polled, fromCommit plumbing.Hash, e gitrepo.LogEntry) {
	r := st.s.Repo()
	ev := Change{Store: name, ULIDs: []string{}, Removed: []string{}, Commit: e.Hash.String()}
	ev.Op, _ = e.Trailer("Hedgehug-Op")
	ev.Author, _ = e.Trailer("Hedgehug-Author")
	from, err1 := r.TreeOf(fromCommit)
	to, err2 := r.TreeOf(e.Hash)
	if err1 != nil || err2 != nil {
		w.sink.TasksChanged(ev)
		return
	}
	changes, err := r.DiffTrees(from, to, "")
	if err != nil {
		w.sink.TasksChanged(ev)
		return
	}
	var schemaCh, viewsCh bool
	for _, c := range changes {
		switch {
		case c.Path == "schema.yaml":
			schemaCh = true
		case strings.HasPrefix(c.Path, model.ViewsDir+"/"):
			viewsCh = true
		}
		if u, ok := model.TaskOfPath(c.Path); ok {
			// A task that became a forward is gone from the list of the
			// store: the file is still there, but not as a task.
			if b, err := r.ReadFile(to, c.Path); c.Deleted || (err == nil && model.IsForward(b)) {
				addOnce(&ev.Removed, u)
			} else {
				addOnce(&ev.ULIDs, u)
			}
			continue
		}
		if u, ok := childTask(c.Path); ok {
			addOnce(&ev.ULIDs, u)
		}
	}
	if len(ev.ULIDs) > 0 || len(ev.Removed) > 0 || (!schemaCh && !viewsCh) {
		w.sink.TasksChanged(ev)
	}
	if schemaCh {
		w.sink.SchemaChanged(name)
	}
	if viewsCh {
		w.sink.ViewsChanged(name)
	}
}

// refresh brings the index of st to HEAD, for the requests that follow, and
// starts bringing its history index to HEAD too.
func (w *Watcher) refresh(st *polled) {
	defer w.buildHistory(st)
	if st.ix != nil && st.ix.Refresh() == nil {
		return
	}
	ix, err := index.Open(st.s)
	if err != nil {
		st.ix = nil
		return
	}
	st.ix = ix
}

// buildHistory builds the history index of st in the background, so that the
// commands that need it find it, and a poll does not wait for it. One build
// runs per store, on a handle of its own (a repository is not safe for
// concurrent use); it goes on, for a few rounds, until the index is at the
// HEAD it sees when it ends. The index is a cache, so a failure is ignored.
func (w *Watcher) buildHistory(st *polled) {
	if !st.histBusy.CompareAndSwap(false, true) {
		return
	}
	st.histDone.Add(1)
	go func() {
		defer st.histDone.Done()
		defer st.histBusy.Store(false)
		s, err := store.Open(st.path)
		if err != nil {
			return
		}
		defer func() { _ = s.Repo().Close() }()
		for range historyRounds {
			h, err := w.openHistory(s, history.Options{Cancel: st.histCancel})
			if err != nil {
				return
			}
			if head, err := s.Repo().Head(); err != nil || head == h.Tip() {
				return
			}
		}
	}()
}

// Wait stops the builds of the history indexes in progress.
func (w *Watcher) Wait() {
	w.pollMu.Lock()
	defer w.pollMu.Unlock()
	w.mu.Lock()
	var all []*polled
	for _, st := range w.stores {
		all = append(all, st)
	}
	w.mu.Unlock()
	for _, st := range all {
		st.stopHistory()
	}
}

// childTask is the task of the comment file at path.
func childTask(path string) (model.ULID, bool) {
	u, _, err := model.ParseCommentPath(path)
	return u, err == nil
}

func addOnce(list *[]string, u model.ULID) {
	if s := u.String(); !slices.Contains(*list, s) {
		*list = append(*list, s)
	}
}

// Snapshot is the HEAD and the sync state of each store, and the items not
// read, for hello.
func (w *Watcher) Snapshot() ([]StoreState, int) {
	// Once a poll has run, hello does not wait for pollMu. Before that it
	// looks again under pollMu: Run's first poll may have run meanwhile, and
	// a second poll that records only would swallow a HEAD move.
	if !w.hasLooked() {
		w.pollMu.Lock()
		if !w.hasLooked() {
			w.pollLocked(false)
		}
		w.pollMu.Unlock()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	out := []StoreState{}
	for name, st := range w.stores {
		out = append(out, StoreState{Store: name, Head: st.head.String(), Sync: st.sync})
	}
	slices.SortFunc(out, func(a, b StoreState) int { return strings.Compare(a.Store, b.Store) })
	return out, w.unread
}

func (w *Watcher) hasLooked() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.looked
}
