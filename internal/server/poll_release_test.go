package server

import (
	"testing"
)

// The session the server gives lets the poller go of a store before a sync
// swaps a rebuilt one in (D26); the poller then opens the store again and
// tells the clients that its tasks changed.
func TestSessionBeforeSwapReleasesTheStore(t *testing.T) {
	ts, ch := pollServer(t)
	_, e, err := ts.session().Entry()
	if err != nil {
		t.Fatal(err)
	}
	var old *polled
	ts.srv.poller.mu.Lock()
	old = ts.srv.poller.stores[e.Name]
	ts.srv.poller.mu.Unlock()
	if old == nil {
		t.Fatal("the poller does not hold the store")
	}

	before := ts.srv.session("").BeforeSwap
	if before == nil {
		t.Fatal("the server's session has no BeforeSwap")
	}
	before(e.Path)

	ev := waitEvent[tasksChanged](t, ch, func(x tasksChanged) bool { return x.Store == e.Name })
	if len(ev.ULIDs) != 0 || len(ev.Removed) != 0 {
		t.Errorf("event = %+v", ev)
	}
	eventually(t, "the poller holds the store opened again", func() bool {
		ts.srv.poller.mu.Lock()
		defer ts.srv.poller.mu.Unlock()
		now := ts.srv.poller.stores[e.Name]
		return now != nil && now != old
	})
}
