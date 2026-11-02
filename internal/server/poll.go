package server

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/history"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// pollInterval is how often the poller reads the HEAD and the sync state of
// each store.
const pollInterval = 300 * time.Millisecond

// historyRounds is how many times a build of the history index goes on when
// HEAD moved meanwhile; the next HEAD move starts another build.
const historyRounds = 3

type tasksChanged struct {
	Store   string   `json:"store"`
	ULIDs   []string `json:"ulids" doc:"The tasks whose file or comments changed"`
	Removed []string `json:"removed" doc:"The tasks whose file is gone (purged)"`
	Commit  string   `json:"commit" doc:"HEAD now"`
	Op      string   `json:"op" doc:"The Hedgehug-Op of HEAD; empty for a commit made by hand"`
	Author  string   `json:"author" doc:"The Hedgehug-Author of HEAD; empty when unknown"`
}

type unreadEvent struct {
	Count int `json:"count" doc:"The items not read, in every store"`
}

type schemaChanged struct {
	Store string `json:"store"`
}

type viewsChanged struct {
	Store string `json:"store"`
}

type storeState struct {
	Store string     `json:"store"`
	Head  string     `json:"head"`
	Sync  syncStatus `json:"sync"`
}

// polled is what the poller knows of a store.
type polled struct {
	path  string
	s     *store.Store
	ix    *index.Index
	head  plumbing.Hash
	sync  syncStatus
	stamp string

	// histBusy is true while a goroutine brings the history index of the
	// store to HEAD; histDone is that goroutine's. Whoever lets go of the
	// store (its directory may be swapped or removed) waits for histDone.
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

// Poller reads the HEAD and the sync state of each store every
// pollInterval, and publishes what changed. It waits in real time: it looks
// at files, which no fake clock moves.
type Poller struct {
	s    *Server
	hub  *Hub
	kick chan struct{}

	interval time.Duration // how long Run waits between looks; pollInterval

	// openHistory builds the history index of a store; history.Open. A seam
	// for the tests.
	openHistory func(*store.Store, history.Options) (*history.History, error)

	pollMu sync.Mutex // one poll at a time; held while reading, not p.mu

	mu       sync.Mutex // the state below
	stores   map[string]*polled
	released map[string]bool // stores swapped for a rebuild, to tell the clients once
	unread   int
	looked   bool   // a poll has run
	scanned  bool   // a poll has read the stores to the end (set under mu)
	lastFail string // the last error of InboxList that was logged
}

func newPoller(s *Server, hub *Hub) *Poller {
	return &Poller{s: s, hub: hub, kick: make(chan struct{}, 1), interval: pollInterval, stores: map[string]*polled{}, openHistory: history.Open}
}

// Release lets go of the store at path, whose directory is about to be
// swapped for a rebuild after a force push (D26): its index and its
// repository files are closed, and the next poll opens the store again and
// tells the clients that its tasks changed. It waits for a poll that is
// reading, and cancels the build of the history index of the store and waits
// for it to end.
func (p *Poller) Release(path string) {
	p.pollMu.Lock()
	defer p.pollMu.Unlock()
	var gone []*polled
	p.mu.Lock()
	for name, st := range p.stores {
		if st.path != path {
			continue
		}
		gone = append(gone, st)
		delete(p.stores, name)
		if p.released == nil {
			p.released = map[string]bool{}
		}
		p.released[name] = true
	}
	p.mu.Unlock()
	// The repository is closed once the build of the history index, which
	// reads it, is over; the wait is made without p.mu, so hello does not wait.
	for _, st := range gone {
		st.stopHistory()
		_ = st.s.Repo().Close()
	}
}

// Kick makes the poller look now (after the server marked items read, or a
// sync ended).
func (p *Poller) Kick() {
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

// Run polls until ctx is done.
func (p *Poller) Run(ctx context.Context) {
	// A build of a history index still writing would outlive the server.
	defer p.waitBuilds()
	for ctx.Err() == nil {
		p.poll(true)
		t := time.NewTimer(p.interval)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-p.kick:
			t.Stop()
		case <-t.C:
		}
	}
}

// poll reads every store once. With publish false it only records (for
// Snapshot before Run has looked). The first sight of a store only records,
// as the HEAD does. The reads that take long (the sync state, the inbox)
// are made without p.mu, so Snapshot does not wait for them.
func (p *Poller) poll(publish bool) {
	p.pollMu.Lock()
	defer p.pollMu.Unlock()
	p.pollLocked(publish)
}

// pollLocked is poll with pollMu held.
func (p *Poller) pollLocked(publish bool) {
	defer func() {
		p.mu.Lock()
		p.looked = true
		p.mu.Unlock()
	}()
	ss := p.s.session("")
	c, _, err := ss.Config()
	if err != nil {
		return
	}
	targets, err := ss.SyncTargets(c, true)
	if errors.Is(err, config.ErrNoStores) {
		// No store is a look to the end too: the one that comes after it (the first one,
		// made from Onboarding) is a store that appeared after the first look.
		targets, err = nil, nil
	}
	if err != nil {
		return
	}
	statuses := map[string]syncStatus{}
	if sts, err := ss.SyncStatus(); err == nil {
		for _, st := range sts {
			statuses[st.Store] = newSyncStatus(st, p.s.loop, c)
		}
	}
	// Only poll writes p.stores and the fields of a polled, and polls run
	// one at a time (pollMu), so it reads them without p.mu; what Snapshot
	// reads is written under p.mu, all together at the end of the loop, so
	// that the loop (index, diffs) does not make hello wait.
	type update struct {
		name  string
		st    *polled
		head  plumbing.Hash
		sync  syncStatus
		fresh bool
	}
	var updates []update
	stamps := map[string]string{} // taken before the inbox is read
	seen := map[string]bool{}
	for _, e := range targets {
		seen[e.Name] = true
		st := p.stores[e.Name]
		fresh := false
		announced := false // the clients were told to read the store again
		if st == nil || st.path != e.Path {
			s, err := p.s.session(e.Name).OpenStore()
			if err != nil {
				continue
			}
			first, err := s.Repo().Head()
			if err != nil {
				continue
			}
			if st != nil {
				st.stopHistory() // the store of the old path is let go
			}
			st = newPolled(e.Path, s)
			fresh = true
			p.mu.Lock()
			swapped := p.released[e.Name]
			delete(p.released, e.Name)
			scanned := p.scanned
			p.mu.Unlock()
			if (swapped || scanned) && publish {
				// The store was made again, or it appeared after the first look: the
				// commits made before this look are not known (a page that made a
				// task in a store just created would never be told), so the clients
				// read it again.
				p.hub.Publish(tasksChanged{Store: e.Name, ULIDs: []string{}, Removed: []string{}, Commit: first.String()})
				announced = true
			}
		}
		head, err := st.s.Repo().Head()
		if err != nil {
			continue
		}
		stamp := inbox.Open(st.s).Stamp()
		if head != st.head {
			if !st.head.IsZero() && publish {
				p.headMoved(e.Name, st, head)
			}
			p.refresh(st)
			stamps[e.Name] = stamp
		}
		u := update{name: e.Name, st: st, head: head, sync: st.sync, fresh: fresh}
		if v, ok := statuses[e.Name]; ok && !reflect.DeepEqual(v, st.sync) {
			u.sync = v
			if publish && (!fresh || announced) {
				p.hub.Publish(v)
			}
		}
		updates = append(updates, u)
		if stamp != st.stamp {
			stamps[e.Name] = stamp
		}
	}
	var dropped []*polled
	p.mu.Lock()
	for _, u := range updates {
		u.st.head, u.st.sync = u.head, u.sync
		p.stores[u.name] = u.st
	}
	p.scanned = true
	for name, st := range p.stores {
		if !seen[name] {
			dropped = append(dropped, st)
			delete(p.stores, name)
		}
	}
	p.mu.Unlock()
	// A store no longer targeted may be removed from the disk next.
	for _, st := range dropped {
		st.stopHistory()
	}
	if len(stamps) == 0 {
		return
	}
	_, items, err := ss.InboxList(false)
	p.mu.Lock()
	defer p.mu.Unlock()
	// The stamps are those from before InboxList, so a file another process
	// wrote meanwhile is seen by the next poll. A failure is not tried again
	// until a file or HEAD changes.
	for name, stamp := range stamps {
		if st := p.stores[name]; st != nil {
			st.stamp = stamp
		}
	}
	if err != nil {
		if msg := err.Error(); msg != p.lastFail {
			p.lastFail = msg
			p.s.logf("inbox: %s", msg)
		}
		return
	}
	p.lastFail = ""
	if len(items) != p.unread {
		p.unread = len(items)
		if publish {
			p.hub.Publish(unreadEvent{Count: p.unread})
		}
	}
}

// headMoved publishes the changes from st.head to head.
func (p *Poller) headMoved(name string, st *polled, head plumbing.Hash) {
	r := st.s.Repo()
	ev := tasksChanged{Store: name, ULIDs: []string{}, Removed: []string{}, Commit: head.String()}
	from, err1 := r.TreeOf(st.head)
	to, err2 := r.TreeOf(head)
	if err1 != nil || err2 != nil {
		p.hub.Publish(ev)
		return
	}
	changes, err := r.DiffTrees(from, to, "")
	if err != nil {
		p.hub.Publish(ev)
		return
	}
	if log, err := r.LogFrom(head, 1); err == nil && len(log) == 1 {
		ev.Op, _ = log[0].Trailer("Hedgehug-Op")
		ev.Author, _ = log[0].Trailer("Hedgehug-Author")
	}
	var schemaCh, viewsCh bool
	for _, c := range changes {
		switch {
		case c.Path == "schema.yaml":
			schemaCh = true
		case strings.HasPrefix(c.Path, model.ViewsDir+"/"):
			viewsCh = true
		}
		if u, ok := ops.TaskOfPath(c.Path); ok {
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
		p.hub.Publish(ev)
	}
	if schemaCh {
		p.hub.Publish(schemaChanged{Store: name})
	}
	if viewsCh {
		p.hub.Publish(viewsChanged{Store: name})
	}
}

// refresh brings the index of st to HEAD, for the requests that follow, and
// starts bringing its history index to HEAD too.
func (p *Poller) refresh(st *polled) {
	defer p.buildHistory(st)
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
func (p *Poller) buildHistory(st *polled) {
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
			h, err := p.openHistory(s, history.Options{Cancel: st.histCancel})
			if err != nil {
				return
			}
			if head, err := s.Repo().Head(); err != nil || head == h.Tip() {
				return
			}
		}
	}()
}

// waitBuilds stops the builds of the history indexes in progress.
func (p *Poller) waitBuilds() {
	p.pollMu.Lock()
	defer p.pollMu.Unlock()
	p.mu.Lock()
	var all []*polled
	for _, st := range p.stores {
		all = append(all, st)
	}
	p.mu.Unlock()
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
func (p *Poller) Snapshot() ([]storeState, int) {
	// Once a poll has run, hello does not wait for pollMu. Before that it
	// looks again under pollMu: Run's first poll may have run meanwhile, and
	// a second poll that records only would swallow a HEAD move.
	if !p.hasLooked() {
		p.pollMu.Lock()
		if !p.hasLooked() {
			p.pollLocked(false)
		}
		p.pollMu.Unlock()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []storeState{}
	for name, st := range p.stores {
		out = append(out, storeState{Store: name, Head: st.head.String(), Sync: st.sync})
	}
	slices.SortFunc(out, func(a, b storeState) int { return strings.Compare(a.Store, b.Store) })
	return out, p.unread
}

func (p *Poller) hasLooked() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.looked
}
