package server

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// A warning that stays is logged once; one that goes away and comes back is
// logged again; the warnings of another store are not touched.
func TestLogWarningsTellsAnEpisodeOnce(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	l := ts.srv.loop
	count := func() int { return strings.Count(ts.log.String(), "sync me: warning: mirror down") }
	l.logWarnings("me", []string{"mirror down"})
	l.logWarnings("other", []string{"mirror down"})
	l.logWarnings("me", []string{"mirror down"})
	if n := count(); n != 1 {
		t.Fatalf("a standing warning was logged %d times", n)
	}
	l.logWarnings("me", nil) // it went away
	l.logWarnings("me", []string{"mirror down"})
	if n := count(); n != 2 {
		t.Errorf("a warning that came back was logged %d times in all, want 2", n)
	}
	if n := strings.Count(ts.log.String(), "sync other: warning: mirror down"); n != 1 {
		t.Errorf("the other store's warning was logged %d times", n)
	}
}

// The background sync of a store tells of a task that lives in two stores,
// once, not at every sync.
func TestBackgroundSyncTellsOfADuplicateOnce(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	ts.addRemote()
	ss := ts.session()
	ss.StoreName = "me"
	r, err := ss.Add(ops.AddInput{Title: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	// the same task, written into team as well
	c, _, err := ts.session().Config()
	if err != nil {
		t.Fatal(err)
	}
	var stores [2]*store.Store
	for i, name := range []string{"me", "team"} {
		e, err := c.Select(name)
		if err != nil {
			t.Fatal(err)
		}
		if stores[i], err = store.Open(e.Path); err != nil {
			t.Fatal(err)
		}
	}
	x, err := stores[0].Task(r.ULID)
	if err != nil {
		t.Fatal(err)
	}
	for w, err := range stores[1].Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		x.Workspace = w.ULID
		break
	}
	x.ID = ""
	if _, err := stores[1].Update(store.Op{Name: "raw", Author: gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}}, func(tx *store.Tx) error { return tx.PutTask(x) }); err != nil {
		t.Fatal(err)
	}

	ts.srv.loop.syncStore("me")
	ts.srv.loop.syncStore("me")
	log := ts.log.String()
	want := r.ULID.String() + " is in "
	if n := strings.Count(log, want); n != 1 {
		t.Errorf("the duplicate is told %d times, want once:\n%s", n, log)
	}
}
