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
	ULIDs   []string `json:"ulids" doc:"The tasks whose file, comments or worklogs changed"`
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

	mu     sync.Mutex
	stores map[string]*polled
	unread int
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
// Snapshot before Run has looked).
func (p *Poller) poll(publish bool) {
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
	p.mu.Lock()
	defer p.mu.Unlock()
	seen := map[string]bool{}
	recount := false
	for _, e := range targets {
		seen[e.Name] = true
		st := p.stores[e.Name]
		if st == nil || st.path != e.Path {
			s, err := p.s.session(e.Name).OpenStore()
			if err != nil {
				continue
			}
			st = &polled{path: e.Path, s: s}
			p.stores[e.Name] = st
		}
		head, err := st.s.Repo().Head()
		if err != nil {
			continue
		}
		if head != st.head {
			if !st.head.IsZero() && publish {
				p.headMoved(e.Name, st, head)
			}
			st.head = head
			p.refresh(st)
			recount = true
		}
		if v, ok := statuses[e.Name]; ok && !reflect.DeepEqual(v, st.sync) {
			st.sync = v
			if publish {
				p.hub.Publish(v)
			}
		}
		if inbox.Open(st.s).Stamp() != st.stamp {
			recount = true
		}
	}
	for name := range p.stores {
		if !seen[name] {
			delete(p.stores, name)
		}
	}
	if recount {
		if _, items, err := ss.InboxList(false); err == nil && len(items) != p.unread {
			p.unread = len(items)
			if publish {
				p.hub.Publish(unreadEvent{Count: p.unread})
			}
		}
		// InboxList writes the files of the boxes (Catch moves the mark), so
		// the stamps are taken after it: else every poll would count again.
		for _, st := range p.stores {
			st.stamp = inbox.Open(st.s).Stamp()
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

// childTask is the task of the comment or worklog file at path.
func childTask(path string) (model.ULID, bool) {
	if u, _, err := model.ParseCommentPath(path); err == nil {
		return u, true
	}
	if u, _, err := model.ParseWorklogPath(path); err == nil {
		return u, true
	}
	return model.ULID{}, false
}

func addOnce(list *[]string, u model.ULID) {
	if s := u.String(); !slices.Contains(*list, s) {
		*list = append(*list, s)
	}
}

// Snapshot is the HEAD and the sync state of each store, and the items not
// read, for hello.
func (p *Poller) Snapshot() ([]storeState, int) {
	p.mu.Lock()
	empty := len(p.stores) == 0
	p.mu.Unlock()
	if empty {
		p.poll(false)
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
