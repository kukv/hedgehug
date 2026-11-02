package server

import (
	"os"
	"testing"
	"time"
)

// The poller builds the history index of a store it looks at, so that the
// commands find it.
func TestPollBuildsTheHistoryIndex(t *testing.T) {
	t.Parallel()
	ts, _ := pollServer(t)
	_, e, err := ts.session().Entry()
	if err != nil {
		t.Fatal(err)
	}
	p := ts.srv.poller
	p.mu.Lock()
	st := p.stores[e.Name]
	p.mu.Unlock()
	if st == nil {
		t.Fatal("the poller does not hold the store")
	}
	eventually(t, "the history index is saved", func() bool {
		_, err := os.Stat(st.s.LocalPath("history"))
		return err == nil
	})
}

// Release does not return while the history index of the store is built: the
// repository is closed after it, and the directory may be swapped next.
func TestReleaseWaitsForTheHistoryBuild(t *testing.T) {
	t.Parallel()
	ts, _ := pollServer(t)
	_, e, err := ts.session().Entry()
	if err != nil {
		t.Fatal(err)
	}
	p := ts.srv.poller
	p.mu.Lock()
	st := p.stores[e.Name]
	p.mu.Unlock()
	if st == nil {
		t.Fatal("the poller does not hold the store")
	}
	st.histDone.Wait() // a build the poll started is over
	st.histDone.Add(1) // a build that is not over

	released := make(chan struct{})
	go func() {
		defer close(released)
		p.Release(e.Path)
	}()
	select {
	case <-released:
		t.Fatal("Release returned while a build was running")
	case <-time.After(100 * time.Millisecond):
	}
	st.histDone.Done()
	select {
	case <-released:
	case <-time.After(10 * time.Second):
		t.Fatal("Release did not return after the build ended")
	}
}
