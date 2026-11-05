package watch_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/watch"
	"github.com/kukv/hedgehug/internal/lib/testenv"
)

// What the recorder keeps of what the watcher tells, in the order it tells.
type (
	storesChanged      struct{}
	schemaChanged      struct{ Store string }
	viewsChanged       struct{ Store string }
	automationsChanged struct{ Store string }
	unreadEvent        struct{ Count int }
)

// recorder is a Sink that keeps every call as a value in ch.
type recorder struct{ ch chan any }

func (r recorder) put(e any) {
	select {
	case r.ch <- e:
	default:
	}
}

func (r recorder) StoresChanged()                     { r.put(storesChanged{}) }
func (r recorder) TasksChanged(c watch.Change)        { r.put(c) }
func (r recorder) SchemaChanged(store string)         { r.put(schemaChanged{Store: store}) }
func (r recorder) ViewsChanged(store string)          { r.put(viewsChanged{Store: store}) }
func (r recorder) AutomationsChanged(store string)    { r.put(automationsChanged{Store: store}) }
func (r recorder) SyncState(v usecase.SyncStatusView) { r.put(v) }
func (r recorder) Unread(count int)                   { r.put(unreadEvent{Count: count}) }

// fixture is a Watcher over a fresh store "me" (usecasetest.New) and what it
// told.
type fixture struct {
	t  *testing.T
	ts *usecasetest.Session
	ch chan any
	w  *watch.Watcher
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, ts: usecasetest.New(t), ch: make(chan any, 4096)}
	f.w = f.newWatcher()
	// A build of a history index still writing would race the removal of the
	// temporary directory.
	t.Cleanup(func() { f.w.Wait() })
	return f
}

// newWatcher is a Watcher that has looked at nothing.
func (f *fixture) newWatcher() *watch.Watcher {
	return watch.New(watch.Options{
		Session: f.sessionOf,
		Sink:    recorder{f.ch},
		Unread: func(ss *usecase.Session) (int, error) {
			_, items, err := notifications.InboxList(ss, false)
			return len(items), err
		},
		Logf: func(string, ...any) {},
	})
}

func (f *fixture) sessionOf(storeName string) *usecase.Session {
	return &usecase.Session{Getenv: f.ts.Getenv, Now: time.Now, Warn: func(string) {}, StoreName: storeName}
}

// session is a usecase.Session on the same config, as the CLI would make.
func (f *fixture) session() *usecase.Session { return f.sessionOf("") }

// pollFixture runs the watcher of a fixture (real clock) and waits for its
// first look; what a test writes must come after it.
func pollFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.run()
	testenv.WaitFor(t, "the first look of the watcher", func() bool { return watch.HeldCount(f.w) > 0 })
	return f
}

// run polls f.w every 10ms until the test ends.
func (f *fixture) run() {
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	w := f.w
	go func() {
		defer close(done)
		// A build of a history index still writing would outlive the test.
		defer w.Wait()
		for ctx.Err() == nil {
			w.Poll(true)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	// Wait for the polls to end: a poll still writing would race the removal
	// of the temporary directory.
	f.t.Cleanup(func() { stop(); <-done })
}

// settle makes the watcher look twice without publishing: the first look
// sets the HEADs and the stamps, the second lets the inbox mark settle.
// pollMu keeps it from overlapping the polls of run.
func (f *fixture) settle() {
	f.w.Poll(false)
	f.w.Poll(false)
}

// twoStores adds the store "team" (prefix TM) and one task to each store.
func (f *fixture) twoStores() {
	f.t.Helper()
	if _, err := stores.StoreInit(f.session(), stores.StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		f.t.Fatal(err)
	}
	for store, title := range map[string]string{"me": "one", "team": "two"} {
		if _, err := tasks.Add(f.sessionOf(store), tasks.AddInput{Title: title}); err != nil {
			f.t.Fatal(err)
		}
	}
}

// notifyMe puts it into the inbox of the store "me", as a sync would, and
// returns its ULID.
func (f *fixture) notifyMe(it inbox.Item) string {
	f.t.Helper()
	s, err := f.sessionOf("me").OpenStore()
	if err != nil {
		f.t.Fatal(err)
	}
	added, err := inbox.Open(s).Add([]inbox.Item{it})
	if err != nil || len(added) != 1 {
		f.t.Fatalf("Add item = %v, %v", added, err)
	}
	return added[0].ULID.String()
}

// overwrittenItem is the item a sync makes when it overwrote the title of the
// task: value is its JSON.
func overwrittenItem(ulid model.ULID, id, value string) inbox.Item {
	return inbox.Item{
		Kind: inbox.Overwritten, Task: ulid, ID: id, Path: model.TaskPath(ulid), Commit: "c1",
		Time:   time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
		Detail: json.RawMessage(`{"field":"title","value":` + value + `}`),
	}
}

// waitEvent returns the first event of type T, skipping the others.
func waitEvent[T any](t *testing.T, ch <-chan any, match func(T) bool) T {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e := <-ch:
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

func drainEvents(ch <-chan any) []any {
	var out []any
	for {
		select {
		case e := <-ch:
			out = append(out, e)
		default:
			return out
		}
	}
}

// countStoresChanged is how many stores.changed are in evs.
func countStoresChanged(evs []any) int {
	n := 0
	for _, e := range evs {
		if _, ok := e.(storesChanged); ok {
			n++
		}
	}
	return n
}
