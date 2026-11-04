package server

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
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

	wake     chan struct{}
	deciding atomic.Bool   // the loop is between waking and waiting (for tests)
	wakes    atomic.Uint64 // requests to look again, counted before they are sent (for tests)
	decided  atomic.Uint64 // wakes that the last decision of the loop had seen (for tests)

	mu      sync.Mutex
	last    map[string]time.Time // store → when its last sync started
	next    map[string]time.Time // store → when the next fetch is due
	running map[string]bool
	active  int             // syncs that run
	idle    chan struct{}   // closed when active falls to 0
	cfgErr  string          // the config.yaml failure last logged; "" while it reads
	told    map[string]bool // warnings of syncs already logged
}

func newLoop(s *Server, clock Clock, visible func() bool) *Loop {
	l := &Loop{
		s: s, clock: clock, visible: visible, onState: func(string) {},
		wake: make(chan struct{}, 1),
		last: map[string]time.Time{}, next: map[string]time.Time{}, running: map[string]bool{}, told: map[string]bool{},
	}
	l.syncFn = l.syncStore
	return l
}

// Wake makes the loop look again now: after a write, or when the GUI shows
// or hides.
func (l *Loop) Wake() {
	l.wakes.Add(1)
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// Run runs the loop until ctx is done.
func (l *Loop) Run(ctx context.Context) {
	for ctx.Err() == nil {
		seen := l.wakes.Load()
		l.deciding.Store(true)
		wait := l.step()
		timer := l.clock.After(wait)
		l.decided.Store(seen)
		l.deciding.Store(false)
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
	sch, ok := l.targets()
	if !ok {
		return time.Minute // config.yaml does not read; look again later
	}
	now := l.clock.Now()
	interval := l.FetchInterval(sch.Interval)
	wait := interval
	for _, name := range sch.Stores {
		due := l.due(name, now, interval, sch.PushInterval)
		if !due.After(now) {
			l.start(name)
			continue
		}
		wait = min(wait, due.Sub(now))
	}
	return wait
}

// FetchInterval is the time between fetches: interval (sync.interval) while
// the GUI is on screen, hiddenInterval at least while it is not.
func (l *Loop) FetchInterval(interval time.Duration) time.Duration {
	if !l.visible() {
		return max(interval, hiddenInterval)
	}
	return interval
}

// noteNext records when name is fetched next, from its last sync.
func (l *Loop) noteNext(name string) {
	ss := l.s.session("")
	defer ss.Close()
	iv, err := ss.SyncIntervals()
	if err != nil {
		l.configFailed(err)
		return
	}
	l.configFailed(nil)
	l.mu.Lock()
	if last, ok := l.last[name]; ok {
		l.next[name] = last.Add(l.FetchInterval(iv.Interval))
	}
	l.mu.Unlock()
}

// configFailed logs a config.yaml that does not read once for as long as the
// message stays the same, and forgets it when it reads again (err nil).
func (l *Loop) configFailed(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err == nil {
		l.cfgErr = ""
		return
	}
	if m := err.Error(); m != l.cfgErr {
		l.cfgErr = m
		l.s.logf("sync: %v", err)
	}
}

// targets are the intervals and the stores with a remote, as config.yaml has
// them now; false when it does not read.
func (l *Loop) targets() (replication.Schedule, bool) {
	ss := l.s.session("")
	defer ss.Close()
	sch, err := replication.LoadSchedule(ss)
	if err != nil {
		l.configFailed(err)
		return replication.Schedule{}, false
	}
	l.configFailed(nil)
	return sch, true
}

// due is when e is to be synced next: now for one never synced, the fetch
// interval after its last sync, or push after its first write that is not
// pushed, but not sooner than push after its last sync.
func (l *Loop) due(name string, now time.Time, interval, push time.Duration) time.Time {
	due, synced, running := l.schedule(name, l.pendingSince(name), interval, push)
	switch {
	case running:
		return now.Add(interval) // looked at again when it ends
	case !synced:
		return now
	}
	l.mu.Lock()
	l.next[name] = l.last[name].Add(interval)
	l.mu.Unlock()
	return due
}

// schedule is when the store name is synced next, for a store that has been
// synced and does not run: the fetch interval after its last sync, or, when
// pending (the first write not pushed) is not zero, push after it, but not
// sooner than push after the last sync and not later than the fetch. It is
// the one place that has this rule; due and the status of the sync use it.
func (l *Loop) schedule(name string, pending time.Time, interval, push time.Duration) (due time.Time, synced, running bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	last, synced := l.last[name]
	if running = l.running[name]; running || !synced {
		return time.Time{}, synced, running
	}
	due = last.Add(interval)
	if !pending.IsZero() {
		due = minTime(due, maxTime(pending.Add(push), last.Add(push)))
	}
	return due, true, false
}

// PushAt is when the loop pushes the writes of name that wait since pending:
// what due makes of them. False for a store not synced yet or running, which
// the loop syncs, and so pushes, now.
func (l *Loop) PushAt(name string, pending time.Time, interval, push time.Duration) (time.Time, bool) {
	due, synced, running := l.schedule(name, pending, interval, push)
	return due, synced && !running
}

// pendingSince is the first write of the store name not pushed; zero when
// there is none or sync.json does not read.
func (l *Loop) pendingSince(name string) time.Time {
	ss := l.s.session(name)
	defer ss.Close()
	return replication.PendingSince(ss)
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
	l.noteNext(name) // while it runs, the next fetch is a whole interval after its start
	go func() {
		fn(name)
		l.mu.Lock()
		l.running[name] = false
		l.active--
		if l.active == 0 {
			close(l.idle)
		}
		l.wakes.Add(1) // the Wake below is owed from here on, so the loop is not parked
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
	defer ss.Close()
	rep, err := replication.SyncOne(ss)
	if err == nil {
		err = rep.Err
	}
	if err != nil {
		l.s.logf("sync %s: %s", name, usecase.Describe(err))
	}
	if rep.Store == "" {
		return // Config or Select failed: no sync ran, and the warnings told stay told
	}
	l.logWarnings(name, rep.Warnings)
}

// logWarnings logs the warnings of a sync of the store name that the sync
// before did not have: one that stays is told once, one that goes away and
// comes back is told again.
func (l *Loop) logWarnings(name string, warnings []string) {
	prefix := name + "\x00"
	l.mu.Lock()
	now := map[string]bool{}
	var fresh []string
	for _, w := range warnings {
		key := prefix + w
		now[key] = true
		if !l.told[key] {
			l.told[key] = true
			fresh = append(fresh, w)
		}
	}
	for key := range l.told {
		if strings.HasPrefix(key, prefix) && !now[key] {
			delete(l.told, key)
		}
	}
	l.mu.Unlock()
	for _, w := range fresh {
		l.s.logf("sync %s: warning: %s", name, w)
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
		if sch, ok := l.targets(); ok {
			for _, name := range sch.Stores {
				if !l.pendingSince(name).IsZero() {
					pending = true
					l.start(name) // nothing new when one runs; it is waited for below
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

// parked reports whether the loop waits with nothing left to look at: no
// sync runs, every wake has been seen by a decision that ended, and the loop
// is not deciding (tests). A wake counts from when it is owed, not when it is
// received: a Wake that is about to be sent, or that the loop has taken from
// its channel and not yet started to decide on, leaves it unparked.
func (l *Loop) parked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active == 0 && l.decided.Load() == l.wakes.Load() && !l.deciding.Load()
}
