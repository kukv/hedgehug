package server

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
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
	r, err := tasks.Add(ss, tasks.AddInput{Title: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	// the same task, written into team as well
	x := ts.uc.TaskByULID("me", r.ULID)
	x.Workspace = ts.uc.WorkspaceNamed("team", "team")
	x.ID = ""
	ts.uc.MustEdit("team", "raw", usecasetest.AliceAuthor, func(tx *usecasetest.Tx) error { return tx.PutTask(x) })

	ts.srv.loop.syncStore("me")
	ts.srv.loop.syncStore("me")
	log := ts.log.String()
	want := r.ULID.String() + " is in "
	if n := strings.Count(log, want); n != 1 {
		t.Errorf("the duplicate is told %d times, want once:\n%s", n, log)
	}
}
