package server

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/watch"
)

// pollInterval is how often the poller reads the HEAD and the sync state of
// each store.
const pollInterval = 300 * time.Millisecond

type tasksChanged struct {
	Store   string   `json:"store"`
	ULIDs   []string `json:"ulids" doc:"The tasks whose file or comments changed"`
	Removed []string `json:"removed" doc:"The tasks whose file is gone (purged)"`
	Commit  string   `json:"commit" doc:"The commit of the change: HEAD, or one of the commits since the last look"`
	Op      string   `json:"op" doc:"The Hedgehug-Op of the commit; empty for a commit made by hand"`
	Author  string   `json:"author" doc:"The Hedgehug-Author of the commit; empty when unknown"`
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
	Store string                 `json:"store"`
	Head  string                 `json:"head"`
	Sync  usecase.SyncStatusView `json:"sync"`
}

// hubSink publishes what the watcher finds, in the order it finds it.
type hubSink struct{ hub *Hub }

func (h hubSink) StoresChanged()                     { h.hub.Publish(storesChanged{}) }
func (h hubSink) TasksChanged(c watch.Change)        { h.hub.Publish(tasksChanged(c)) }
func (h hubSink) SchemaChanged(store string)         { h.hub.Publish(schemaChanged{Store: store}) }
func (h hubSink) ViewsChanged(store string)          { h.hub.Publish(viewsChanged{Store: store}) }
func (h hubSink) SyncState(v usecase.SyncStatusView) { h.hub.Publish(v) }
func (h hubSink) Unread(count int)                   { h.hub.Publish(unreadEvent{Count: count}) }

// Poller reads the HEAD and the sync state of each store every
// pollInterval, and publishes what changed (the watcher finds it). It waits
// in real time: it looks at files, which no fake clock moves.
type Poller struct {
	w    *watch.Watcher
	hub  *Hub
	kick chan struct{}

	interval time.Duration // how long Run waits between looks; pollInterval
}

func newPoller(s *Server, hub *Hub) *Poller {
	w := watch.New(watch.Options{
		Session:  s.session,
		Schedule: s.loop,
		Sink:     hubSink{hub},
		Unread: func(ss *usecase.Session) (int, error) {
			_, items, err := notifications.InboxList(ss, false)
			return len(items), err
		},
		Logf: s.logf,
	})
	return &Poller{w: w, hub: hub, kick: make(chan struct{}, 1), interval: pollInterval}
}

// Release lets go of the store at path, whose directory is about to be
// rebuilt after a force push (D26); see watch.Watcher.Release.
func (p *Poller) Release(path string) { p.w.Release(path) }

// ReleaseFor lets go of the store at path and runs fn before any poll can
// look again; see watch.Watcher.ReleaseFor.
func (p *Poller) ReleaseFor(path string, fn func() error) error { return p.w.ReleaseFor(path, fn) }

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

// poll reads every store once; the tests call it by hand.
func (p *Poller) poll(publish bool) { p.w.Poll(publish) }

func (p *Poller) waitBuilds() { p.w.Wait() }

// snapshot is the HEAD and the sync state of each store, and the items not
// read, for hello.
func (p *Poller) snapshot() ([]storeState, int) {
	states, unread := p.w.Snapshot()
	out := make([]storeState, len(states))
	for i, st := range states {
		out[i] = storeState(st)
	}
	return out, unread
}
