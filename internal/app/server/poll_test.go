package server

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

// pollServer runs the poller of a test server (real clock) and returns the
// events it publishes.
func pollServer(t *testing.T) (*testServer, <-chan sseEvent) {
	t.Helper()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	ts.srv.poller.interval = 10 * time.Millisecond
	go func() {
		defer close(done)
		ts.srv.poller.Run(ctx)
	}()
	// Wait for Run to return: a poll still writing would race the removal
	// of the temporary directory.
	t.Cleanup(func() { stop(); cancel(); <-done })
	// The first look sets the HEADs; what a test writes must come after it.
	eventually(t, "the first look of the poller", func() bool {
		stores, _ := ts.srv.poller.snapshot()
		return len(stores) > 0
	})
	return ts, ch
}

// settle makes the poller look twice without publishing: the first look sets
// the HEADs and the stamps, the second lets the inbox mark settle. pollMu
// keeps it from overlapping Run.
func (p *Poller) settle() {
	p.poll(false)
	p.poll(false)
}

// waitEvent returns the first event of type T, skipping the others.
func waitEvent[T any](t *testing.T, ch <-chan sseEvent, match func(T) bool) T {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatal("the hub dropped the subscription")
			}
			if v, ok := e.(T); ok && (match == nil || match(v)) {
				return v
			}
		case <-timeout:
			var zero T
			t.Fatalf("no %T event", zero)
			return zero
		}
	}
}

func TestPollSyncState(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	ts.srv.poller.settle()
	// push_at is told for a store the loop has synced.
	ts.srv.loop.Synced("me")
	ts.uc.MarkPending("me", time.Now())
	e := waitEvent[usecase.SyncStatusView](t, ch, func(e usecase.SyncStatusView) bool { return e.PendingSince != nil })
	if e.Store != "me" {
		t.Errorf("event = %+v", e)
	}
	ts.srv.loop.mu.Lock()
	last := ts.srv.loop.last["me"]
	ts.srv.loop.mu.Unlock()
	if want := maxTime(*e.PendingSince, last).Add(30 * time.Second); e.PushAt == nil || !e.PushAt.Equal(want) {
		t.Errorf("push_at = %v; want %v (30s after the later of pending_since and the last sync)", e.PushAt, want)
	}
	if e.Interval <= 0 || e.PushInterval <= 0 {
		t.Errorf("interval = %v, push_interval = %v; want the sync.* intervals in seconds", e.Interval, e.PushInterval)
	}
}

// holds is whether the poller holds the store named name.
func holds(p *Poller, name string) bool {
	stores, _ := p.snapshot()
	for _, st := range stores {
		if st.Store == name {
			return true
		}
	}
	return false
}
