package watch_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/watch"
	"github.com/kukv/hedgehug/internal/lib/testenv"
)

// A task that became a forward is no longer in the store's list: the watcher
// tells so as removed, not as changed (a refetch would get task.moved).
func TestPollMovedTaskIsRemoved(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.twoStores()
	ss := f.session()
	ss.StoreName = "me"
	moved, err := tasks.Add(ss, tasks.AddInput{Title: "moves"})
	if err != nil {
		t.Fatal(err)
	}
	f.run()
	testenv.WaitFor(t, "the first look of the watcher", func() bool { return watch.HeldCount(f.w) > 1 })
	f.settle()

	if _, err := stores.MoveStore(f.session(), stores.MoveStoreInput{Arg: moved.ULID.String(), To: "team", Yes: true}); err != nil {
		t.Fatal(err)
	}
	// The watcher reads the two stores in no fixed order, so the two events
	// come in either order.
	var e, in watch.Change
	var gotOut, gotIn bool
	waitEvent(t, f.ch, func(ev watch.Change) bool {
		switch {
		case ev.Store == "me" && ev.Op == "move-out":
			e, gotOut = ev, true
		case ev.Store == "team" && ev.Op == "move-in":
			in, gotIn = ev, true
		}
		return gotOut && gotIn
	})
	if !slices.Contains(e.Removed, moved.ULID.String()) || slices.Contains(e.ULIDs, moved.ULID.String()) {
		t.Errorf("move-out event = %+v; want the task removed, not changed", e)
	}
	if !slices.Contains(in.ULIDs, moved.ULID.String()) {
		t.Errorf("move-in event = %+v; want the task changed", in)
	}
}

// The watcher builds the history index of a store it looks at, so that the
// commands find it.
func TestPollBuildsTheHistoryIndex(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	st := watch.HeldStore(f.w, "me")
	if st == nil {
		t.Fatal("the watcher does not hold the store")
	}
	testenv.WaitFor(t, "the history index is saved", func() bool {
		_, err := os.Stat(st.LocalPath("history"))
		return err == nil
	})
}

// Release cancels a build of the history index that is running and returns
// once it ended, before the repository is closed and the store rebuilt.
func TestReleaseCancelsTheHistoryBuild(t *testing.T) {
	t.Parallel()
	f := newFixture(t) // its watcher does not run: Poll is called by hand
	_, e, err := f.session().Entry()
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	watch.SetOpenHistory(f.w, func(_ *store.Store, opt history.Options) (*history.History, error) {
		close(started)
		<-opt.Cancel // a build that only the cancel ends
		return nil, history.ErrCanceled
	})
	f.w.Poll(false) // the first sight of the store starts its build
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the build did not start")
	}

	released := make(chan struct{})
	go func() {
		defer close(released)
		f.w.Release(e.Path)
	}()
	select {
	case <-released:
	case <-time.After(10 * time.Second):
		t.Fatal("Release did not return while a build was running")
	}
}

// A poll does not open a store while a rebuild is under way (rebuild.json),
// whatever its phase: a pack it holds open would stop the prune. It opens
// the store on the poll after the rebuild ended.
func TestPollSkipsAStoreBeingRebuilt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, e, err := f.session().Entry()
	if err != nil {
		t.Fatal(err)
	}
	rec := filepath.Join(e.Path, filepath.FromSlash(store.LocalDir), store.RebuildFile)
	if err := os.MkdirAll(filepath.Dir(rec), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"apply", "prune"} {
		if err := os.WriteFile(rec, []byte(`{"phase":"`+phase+`"}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		f.w.Poll(false)
		if watch.Held(f.w, "me") {
			t.Errorf("the poll opened the store in the %s phase", phase)
		}
	}
	if err := os.Remove(rec); err != nil {
		t.Fatal(err)
	}
	f.w.Poll(false)
	if !watch.Held(f.w, "me") {
		t.Error("the poll did not open the store after the rebuild")
	}
}

// A store held while its rebuild stops in the apply phase gets no history
// index built: its work tree is part old, part new.
func TestPollBuildsNoHistoryInTheApplyPhase(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	var builds atomic.Int32
	watch.SetOpenHistory(f.w, func(*store.Store, history.Options) (*history.History, error) {
		builds.Add(1)
		return nil, history.ErrCanceled
	})
	f.w.Poll(false) // the first sight of the store starts a build
	testenv.WaitFor(t, "the first build", func() bool { return builds.Load() == 1 })
	ss := f.sessionOf("me")
	if _, err := tasks.Add(ss, tasks.AddInput{Title: "moves HEAD"}); err != nil {
		t.Fatal(err)
	}
	_, e, err := ss.Entry()
	if err != nil {
		t.Fatal(err)
	}
	rec := filepath.Join(e.Path, filepath.FromSlash(store.LocalDir), store.RebuildFile)
	if err := os.MkdirAll(filepath.Dir(rec), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rec, []byte(`{"phase":"apply"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.w.Poll(false) // HEAD moved: a build would start
	f.w.Wait()      // and has ended when this returns
	if n := builds.Load(); n != 1 {
		t.Errorf("builds = %d, want 1", n)
	}
}

// A watcher that does not run still waits, when Wait is called as the test
// is cleaned up, for the build of the history index that a poll by hand
// started: the build writes under the temporary directory, which is removed
// right after.
func TestWaitWaitsForTheHistoryBuild(t *testing.T) {
	t.Parallel()
	ended := make(chan struct{})
	t.Run("watcher", func(t *testing.T) {
		f := newFixture(t) // its watcher does not run: Poll is called by hand
		started := make(chan struct{})
		watch.SetOpenHistory(f.w, func(_ *store.Store, opt history.Options) (*history.History, error) {
			defer close(ended)
			close(started)
			<-opt.Cancel // a build that only the cancel ends
			return nil, history.ErrCanceled
		})
		f.w.Poll(false) // the first sight of the store starts its build
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("the build did not start")
		}
	})
	select {
	case <-ended:
	default:
		t.Fatal("the build of the history index outlived the test")
	}
}

// A look that finds config.yaml different from the one before (a store came
// or went, its color, visibility or account changed, or the default store)
// tells the clients; the first look, and a look that finds the same, do not.
// The CLI writes config.yaml the same way, so this is also what tells the
// GUI of hh store set.
func TestPollStoresChanged(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	look := func() []any {
		f.w.Poll(true)
		return drainEvents(f.ch)
	}
	want := func(what string, n int, evs []any) {
		t.Helper()
		if got := countStoresChanged(evs); got != n {
			t.Errorf("%s: %d stores.changed, want %d", what, got, n)
		}
	}

	want("the first look", 0, look())
	want("a look that finds the same", 0, look())

	ss := f.session()
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
	if watch.Held(f.w, "team") {
		t.Error("the watcher still holds the store that went")
	}
	want("nothing changed after it went", 0, look())
}

// ReleaseFor lets go of the store and keeps the polls out until fn is done,
// so that a poll cannot open the store again while its directory is removed.
func TestPollerReleaseForKeepsPollsOut(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.settle()
	_, e, err := f.session().Entry()
	if err != nil {
		t.Fatal(err)
	}

	// The poll starts inside fn, so it can only get in after fn returns. It
	// records whether fn had returned when its poll ended.
	var fnReturned atomic.Bool
	polling, done := make(chan struct{}), make(chan bool, 1)
	err = f.w.ReleaseFor(e.Path, func() error {
		if watch.Held(f.w, e.Name) {
			t.Error("the store is still held inside fn")
		}
		go func() {
			close(polling)
			f.w.Poll(false)
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
	if !watch.Held(f.w, e.Name) {
		t.Error("the poll after ReleaseFor did not open the store again")
	}
	if watch.Released(f.w, e.Name) {
		t.Error("a store taken out for good is still marked as rebuilt")
	}
}
