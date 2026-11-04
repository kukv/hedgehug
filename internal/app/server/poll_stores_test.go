package server

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
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
	if _, err := ss.StoreInit(usecase.StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	want("a store came", 1, look())
	want("nothing changed after it", 0, look())

	if _, err := ss.StoreSet("team", new("teal"), nil); err != nil {
		t.Fatal(err)
	}
	want("a color was set", 1, look())

	if _, err := ss.StoreSet("team", nil, new(true)); err != nil {
		t.Fatal(err)
	}
	want("a store was hidden", 1, look())

	if _, err := ss.StoreDefault("team"); err != nil {
		t.Fatal(err)
	}
	want("the default store changed", 1, look())

	if _, err := ss.AuthAdd(usecase.AuthAddInput{Name: "bob", Login: "bob"}); err != nil {
		t.Fatal(err)
	}
	want("an account came", 1, look())

	if _, err := ss.StoreRemove("team", true); err != nil {
		t.Fatal(err)
	}
	evs := look()
	want("a store went", 1, evs)
	for _, e := range evs {
		if st, ok := e.(syncStatus); ok && st.Store == "team" {
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

	started, done := make(chan struct{}), make(chan struct{})
	go func() {
		<-started
		p.poll(false)
		close(done)
	}()
	err = p.ReleaseFor(e.Path, func() error {
		p.mu.Lock()
		_, held := p.stores[e.Name]
		p.mu.Unlock()
		if held {
			t.Error("the store is still held inside fn")
		}
		close(started)
		select {
		case <-done:
			t.Error("a poll ran while fn ran")
		case <-time.After(150 * time.Millisecond):
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-done
	p.mu.Lock()
	swapped := p.released[e.Name]
	p.mu.Unlock()
	if swapped {
		t.Error("a store taken out for good is still marked as swapped")
	}
}
