package server

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// pollInterval is how often the poller reads the HEAD and the sync state of
// each store.
const pollInterval = 300 * time.Millisecond

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
}

// Poller reads the HEAD and the sync state of each store every
// pollInterval, and publishes what changed. It waits in real time: it looks
// at files, which no fake clock moves.
type Poller struct {
	s    *Server
	hub  *Hub
	kick chan struct{}

	pollMu sync.Mutex // one poll at a time; held while reading, not p.mu

	mu       sync.Mutex // the state below
	stores   map[string]*polled
	unread   int
	looked   bool   // a poll has run
	lastFail string // the last error of InboxList that was logged
}

func newPoller(s *Server, hub *Hub) *Poller {
	return &Poller{s: s, hub: hub, kick: make(chan struct{}, 1), stores: map[string]*polled{}}
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
	for ctx.Err() == nil {
		p.poll(true)
		t := time.NewTimer(pollInterval)
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
	if err != nil {
		return
	}
	statuses := map[string]syncStatus{}
	if sts, err := ss.SyncStatus(); err == nil {
		for _, st := range sts {
			v := syncStatus{StoreStatus: st}
			if t, ok := p.s.loop.NextFetch(st.Store); ok {
				v.NextFetch = &t
			}
			statuses[st.Store] = v
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
		if st == nil || st.path != e.Path {
			s, err := p.s.session(e.Name).OpenStore()
			if err != nil {
				continue
			}
			st = &polled{path: e.Path, s: s}
			fresh = true
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
			if publish && !fresh {
				p.hub.Publish(v)
			}
		}
		updates = append(updates, u)
		if stamp != st.stamp {
			stamps[e.Name] = stamp
		}
	}
	p.mu.Lock()
	for _, u := range updates {
		u.st.head, u.st.sync = u.head, u.sync
		p.stores[u.name] = u.st
	}
	for name := range p.stores {
		if !seen[name] {
			delete(p.stores, name)
		}
	}
	p.mu.Unlock()
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
			if c.Deleted {
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

// refresh brings the index of st to HEAD, for the requests that follow.
func (p *Poller) refresh(st *polled) {
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
