package server

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
)

// A look that finds config.yaml different from the one before (a store came
// or went, its color, visibility or account changed, or the default store)
// tells the clients; the first look, and a look that finds the same, do not.
// The CLI writes config.yaml the same way, so this is also what tells the
// GUI of hh store set.
func TestPollStoresChanged(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	t.Cleanup(cancel)
	p := ts.srv.poller
	look := func() []sseEvent {
		p.poll(true)
		return drainEvents(ch)
	}
	want := func(what string, n int, evs []sseEvent) {
		t.Helper()
		if got := countStoresChanged(evs); got != n {
			t.Errorf("%s: %d stores.changed, want %d", what, got, n)
		}
	}

	want("the first look", 0, look())
	want("a look that finds the same", 0, look())

	ss := ts.session()
	if _, err := stores.StoreInit(ss, stores.StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	want("a store came", 1, look())
	want("nothing changed after it", 0, look())

	if _, err := stores.StoreSet(ss, "team", new("teal"), nil); err != nil {
		t.Fatal(err)
	}
	want("a color was set", 1, look())

	if _, err := stores.StoreSet(ss, "team", nil, new(true)); err != nil {
		t.Fatal(err)
	}
	want("a store was hidden", 1, look())

	if _, err := stores.StoreDefault(ss, "team"); err != nil {
		t.Fatal(err)
	}
	want("the default store changed", 1, look())

	if _, err := accounts.AuthAdd(ss, accounts.AuthAddInput{Name: "bob", Login: "bob"}); err != nil {
		t.Fatal(err)
	}
	want("an account came", 1, look())

	if _, err := stores.StoreRemove(ss, "team", true); err != nil {
		t.Fatal(err)
	}
	evs := look()
	want("a store went", 1, evs)
	for _, e := range evs {
		if st, ok := e.(usecase.SyncStatusView); ok && st.Store == "team" {
			t.Errorf("sync.state of the store that went: %+v", st)
		}
	}
	p.mu.Lock()
	_, held := p.stores["team"]
	p.mu.Unlock()
	if held {
		t.Error("the poller still holds the store that went")
	}
	want("nothing changed after it went", 0, look())
}

// ReleaseFor lets go of the store and keeps the polls out until fn is done,
// so that a poll cannot open the store again while its directory is removed.
func TestPollerReleaseForKeepsPollsOut(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.srv.poller
	p.settle()
	_, e, err := ts.session().Entry()
	if err != nil {
		t.Fatal(err)
	}

	// The poll starts inside fn, so it can only get in after fn returns. It
	// records whether fn had returned when its poll ended.
	var fnReturned atomic.Bool
	polling, done := make(chan struct{}), make(chan bool, 1)
	err = p.ReleaseFor(e.Path, func() error {
		p.mu.Lock()
		_, held := p.stores[e.Name]
		p.mu.Unlock()
		if held {
			t.Error("the store is still held inside fn")
		}
		go func() {
			close(polling)
			p.poll(false)
			done <- fnReturned.Load()
		}()
		<-polling
		for range 1000 { // give the poll every chance to run before fn ends
			runtime.Gosched()
		}
		fnReturned.Store(true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case after := <-done:
		if !after {
			t.Error("a poll ran while fn ran")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the poll never ended")
	}
	p.mu.Lock()
	_, opened := p.stores[e.Name]
	p.mu.Unlock()
	if !opened {
		t.Error("the poll after ReleaseFor did not open the store again")
	}
	p.mu.Lock()
	swapped := p.released[e.Name]
	p.mu.Unlock()
	if swapped {
		t.Error("a store taken out for good is still marked as swapped")
	}
}
