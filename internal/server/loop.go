package server

import (
	"context"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/ops"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// hiddenInterval is the shortest time between fetches while no tab shows
// the GUI.
const hiddenInterval = 10 * time.Minute

// Loop syncs the stores with a remote: once at the start, then every
// sync.interval while the GUI is on screen (hiddenInterval at least while it
// is not), and push_interval after a write.
type Loop struct {
	s       *Server
	clock   Clock
	visible func() bool
	onState func(store string)
	syncFn  func(store string) // the sync of one store; tests replace it

	wake chan struct{}

	mu      sync.Mutex
	last    map[string]time.Time // store → when its last sync started
	next    map[string]time.Time // store → when the next fetch is due
	running map[string]bool
	active  int           // syncs that run
	idle    chan struct{} // closed when active falls to 0
}

func newLoop(s *Server, clock Clock, visible func() bool) *Loop {
	l := &Loop{
		s: s, clock: clock, visible: visible, onState: func(string) {},
		wake: make(chan struct{}, 1),
		last: map[string]time.Time{}, next: map[string]time.Time{}, running: map[string]bool{},
	}
	l.syncFn = l.syncStore
	return l
}

// Wake makes the loop look again now: after a write, or when the GUI shows
// or hides.
func (l *Loop) Wake() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// Run runs the loop until ctx is done.
func (l *Loop) Run(ctx context.Context) {
	for ctx.Err() == nil {
		began := l.clock.Now()
		wait := l.step()
		timer := l.clock.After(wait)
		// A clock that moved while step decided would make timer late by as
		// much; decide again.
		if l.clock.Now().Sub(began) > time.Second {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-l.wake:
		case <-timer:
		}
	}
}

// step starts the syncs that are due and returns how long to wait for the
// next one.
func (l *Loop) step() time.Duration {
	c, targets := l.targets()
	if c == nil {
		return time.Minute // config.yaml does not read; look again later
	}
	now := l.clock.Now()
	interval := l.fetchInterval(c)
	wait := interval
	for _, e := range targets {
		due := l.due(e, now, interval, c.Sync.PushInterval)
		if !due.After(now) {
			l.start(e.Name)
			continue
		}
		wait = min(wait, due.Sub(now))
	}
	return wait
}

// fetchInterval is the time between fetches: sync.interval while the GUI is
// on screen, hiddenInterval at least while it is not.
func (l *Loop) fetchInterval(c *config.Config) time.Duration {
	if !l.visible() {
		return max(c.Sync.Interval, hiddenInterval)
	}
	return c.Sync.Interval
}

// noteNext records when name is fetched next, from its last sync.
func (l *Loop) noteNext(name string) {
	c, _, err := l.s.session("").Config()
	if err != nil {
		return
	}
	l.mu.Lock()
	if last, ok := l.last[name]; ok {
		l.next[name] = last.Add(l.fetchInterval(c))
	}
	l.mu.Unlock()
}

// targets are the stores with a remote, as config.yaml has them now.
func (l *Loop) targets() (*config.Config, []config.StoreEntry) {
	ss := l.s.session("")
	c, _, err := ss.Config()
	if err != nil {
		l.s.logf("sync: %v", err)
		return nil, nil
	}
	all, err := ss.SyncTargets(c, true)
	if err != nil {
		l.s.logf("sync: %v", err)
		return nil, nil
	}
	var out []config.StoreEntry
	for _, e := range all {
		if ops.SyncsInBackground(e) {
			out = append(out, e)
		}
	}
	return c, out
}

// due is when e is to be synced next: now for one never synced, the fetch
// interval after its last sync, or push after its first write that is not
// pushed, but not sooner than push after its last sync.
func (l *Loop) due(e config.StoreEntry, now time.Time, interval, push time.Duration) time.Time {
	l.mu.Lock()
	last, synced := l.last[e.Name]
	running := l.running[e.Name]
	l.mu.Unlock()
	if running {
		return now.Add(interval) // looked at again when it ends
	}
	if !synced {
		return now
	}
	due := last.Add(interval)
	if pending := l.pendingSince(e); !pending.IsZero() {
		due = minTime(due, maxTime(pending.Add(push), last.Add(push)))
	}
	l.mu.Lock()
	l.next[e.Name] = last.Add(interval)
	l.mu.Unlock()
	return due
}

// pendingSince is the first write of e not pushed; zero when there is none
// or sync.json does not read.
func (l *Loop) pendingSince(e config.StoreEntry) time.Time {
	s, err := l.s.session(e.Name).OpenStore()
	if err != nil {
		return time.Time{}
	}
	st, err := hhsync.ReadState(s)
	if err != nil {
		return time.Time{}
	}
	return st.PendingSince
}

// start runs the sync of name in its own goroutine, unless one runs.
func (l *Loop) start(name string) {
	l.mu.Lock()
	if l.running[name] {
		l.mu.Unlock()
		return
	}
	l.running[name] = true
	l.last[name] = l.clock.Now()
	fn := l.syncFn
	if l.active == 0 {
		l.idle = make(chan struct{})
	}
	l.active++
	l.mu.Unlock()
	go func() {
		fn(name)
		l.mu.Lock()
		l.running[name] = false
		l.active--
		if l.active == 0 {
			close(l.idle)
		}
		l.mu.Unlock()
		l.noteNext(name)
		l.onState(name)
		l.Wake()
	}()
}

// syncStore is the sync of one store, as the background sync of the CLI
// does it.
func (l *Loop) syncStore(name string) {
	ss := l.s.session(name)
	c, _, err := ss.Config()
	if err != nil {
		l.s.logf("sync %s: %v", name, err)
		return
	}
	e, err := c.Select(name)
	if err != nil {
		l.s.logf("sync %s: %v", name, err)
		return
	}
	rep, err := ss.SyncStore(c, e, false)
	if err == nil {
		err = rep.Err
	}
	if err != nil {
		l.s.logf("sync %s: %s", name, detailOf(err))
	}
}

// setSyncFn replaces the sync of one store (tests).
func (l *Loop) setSyncFn(fn func(store string)) {
	l.mu.Lock()
	l.syncFn = fn
	l.mu.Unlock()
}

// Synced notes a sync made outside the loop (POST /api/sync).
func (l *Loop) Synced(name string) {
	l.mu.Lock()
	l.last[name] = l.clock.Now()
	l.mu.Unlock()
	l.noteNext(name)
	l.Wake()
}

// NextFetch is when the loop fetches name next; false for a store it has
// not synced yet.
func (l *Loop) NextFetch(name string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.next[name]
	return t, ok
}

// Drain syncs the stores with a write not pushed, and waits for them and
// for the syncs that run, timeout at most. A sync that ran already when
// Drain began may have started before the write; so once they end, the
// stores still pending are synced once more.
func (l *Loop) Drain(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for range 2 {
		pending := false
		if _, targets := l.targets(); targets != nil {
			for _, e := range targets {
				if !l.pendingSince(e).IsZero() {
					pending = true
					l.start(e.Name) // nothing new when one runs; it is waited for below
				}
			}
		}
		if !l.waitUntil(deadline) {
			l.s.logf("sync: stopped waiting for the push after %v", timeout)
			return
		}
		if !pending {
			return
		}
	}
}

// waitUntil waits for the syncs that run, until deadline; false when it
// came first.
func (l *Loop) waitUntil(deadline time.Time) bool {
	l.mu.Lock()
	idle := l.idle
	if l.active == 0 {
		idle = nil
	}
	l.mu.Unlock()
	if idle == nil {
		return true
	}
	select {
	case <-idle:
		return true
	case <-time.After(time.Until(deadline)):
		return false
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
