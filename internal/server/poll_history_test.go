package server

import (
	"os"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/history"
	"github.com/kukv/hedgehug/internal/store"
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

// Release cancels a build of the history index that is running and returns
// once it ended, before the repository is closed and the directory swapped.
func TestReleaseCancelsTheHistoryBuild(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t) // its poller does not run: poll is called by hand
	_, e, err := ts.session().Entry()
	if err != nil {
		t.Fatal(err)
	}
	p := ts.srv.poller
	started := make(chan struct{})
	p.openHistory = func(_ *store.Store, opt history.Options) (*history.History, error) {
		close(started)
		<-opt.Cancel // a build that only the cancel ends
		return nil, history.ErrCanceled
	}
	p.poll(false) // the first sight of the store starts its build
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the build did not start")
	}

	released := make(chan struct{})
	go func() {
		defer close(released)
		p.Release(e.Path)
	}()
	select {
	case <-released:
	case <-time.After(10 * time.Second):
		t.Fatal("Release did not return while a build was running")
	}
}

// A test server whose poller does not run still waits, as it is cleaned up,
// for the build of the history index that a poll by hand started: the build
// writes under the temporary directory, which is removed right after.
func TestTestServerWaitsForTheHistoryBuild(t *testing.T) {
	t.Parallel()
	ended := make(chan struct{})
	t.Run("server", func(t *testing.T) {
		ts := newTestServer(t) // its poller does not run: poll is called by hand
		p := ts.srv.poller
		started := make(chan struct{})
		p.openHistory = func(_ *store.Store, opt history.Options) (*history.History, error) {
			defer close(ended)
			close(started)
			<-opt.Cancel // a build that only the cancel ends
			return nil, history.ErrCanceled
		}
		p.poll(false) // the first sight of the store starts its build
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("the build did not start")
		}
	})
	select {
	case <-ended:
	default:
		t.Fatal("the build of the history index outlived the test server")
	}
}
