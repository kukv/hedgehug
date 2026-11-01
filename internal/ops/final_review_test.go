package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/query"
	"github.com/kukv/hedgehug/internal/store"
)

// addGoneStore sets up a store called gone in config.yaml at a path where
// there is no store, with a remote when withRemote.
func addGoneStore(t *testing.T, ts *testSession, withRemote bool) {
	t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	var remotes []string
	if withRemote {
		remotes = []string{filepath.Join(t.TempDir(), "remote.git")}
	}
	if err := c.AddStore("gone", filepath.Join(t.TempDir(), "gone"), remotes); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.dropMount()
	ts.warns = nil
}

func countContaining(warns []string, sub string) int {
	var n int
	for _, w := range warns {
		if strings.Contains(w, sub) {
			n++
		}
	}
	return n
}

// hh use --store looks in that store only.
func TestUseWithStoreLooksInThatStoreOnly(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSessionWith(t, "API", "API")
	idA := addProjectIn(t, ts, "org-a", "Release A")
	idB := addProjectIn(t, ts, "org-b", "Release B")
	if idA != idB {
		t.Fatalf("ids %s and %s; want the same id in both stores", idA, idB)
	}
	// without --store the id is in both
	if _, err := ts.Use(idA); KindOf(err) != KindAmbiguous {
		t.Fatalf("Use(%s) err = %v; want ambiguous", idA, err)
	}
	ts.StoreName = "org-b"
	r, err := ts.Use(idA)
	if err != nil || r.Store != "org-b" || r.Project.Title != "Release B" {
		t.Fatalf("Use(%s) --store org-b = %+v, %v; want org-b's project", idA, r, err)
	}
	// the same store spelled in the id is fine
	if r, err = ts.Use("org-b:" + idA); err != nil || r.Store != "org-b" {
		t.Errorf("Use(org-b:%s) --store org-b = %+v, %v", idA, r, err)
	}
	// another store's name is a usage error
	_, err = ts.Use("org-a:" + idA)
	wantCode(t, "Use(org-a:...) --store org-b", err, KindUsage, "store.conflict")
	// a project that is only in the other store is not found
	ts.StoreName = "org-a"
	other := addProjectIn(t, ts, "org-b", "Only B")
	_, err = ts.Use(other)
	wantCode(t, "Use(project of org-b) --store org-a", err, KindNotFound, "task.not_found")
	ts.StoreName = "nope"
	_, err = ts.Use(idA)
	wantCode(t, "Use --store nope", err, KindNotFound, "store.not_found")
}

// A command about one store does not warn of a store it is not about.
func TestStoreNameKeepsTheWarningsOfOtherStoresQuiet(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	addGoneStore(t, ts, false)
	ts.StoreName = "org-a"
	if _, err := ts.List(&query.Query{}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Mount(); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(ts.warns, "cannot open gone"); n != 0 {
		t.Errorf("--store org-a warned %d times of gone: %q", n, ts.warns)
	}
	// without --store it still does, once
	ts.StoreName = ""
	if _, err := ts.List(&query.Query{}); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(ts.warns, "cannot open gone"); n != 1 {
		t.Errorf("List warned %d times of gone: %q", n, ts.warns)
	}
}

// A sync that fails for a store that does not open says so in its report;
// the cross-store check of the other stores does not say it again.
func TestSyncDoesNotWarnTwiceOfAStoreItReports(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	addGoneStore(t, ts, true)
	rs, err := ts.Sync(true, true)
	if err == nil {
		t.Fatal("Sync of a store that does not open succeeded")
	}
	var failed bool
	for _, r := range rs {
		if r.Store == "gone" && r.Err != nil {
			failed = true
		}
	}
	if !failed {
		t.Fatalf("reports = %+v; want gone to have failed", rs)
	}
	if n := countContaining(ts.warns, "cannot open gone"); n != 0 {
		t.Errorf("warnings %q repeat what the report of gone says", ts.warns)
	}
	// the silence was for the sync only
	ts.warns = nil
	if _, err := ts.List(&query.Query{}); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(ts.warns, "cannot open gone"); n != 1 {
		t.Errorf("List after the sync warned %d times of gone: %q", n, ts.warns)
	}
}

// inbox leaves out a store that does not open, with a warning, but not the
// one --store names.
func TestInboxSkipsAStoreThatDoesNotOpen(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"list", "read", "revert"} {
		ts := newTwoStoreSession(t)
		addGoneStore(t, ts, false)
		var err error
		switch name {
		case "list":
			_, _, err = ts.InboxList(false)
		case "read":
			_, err = ts.InboxRead(nil, true)
		case "revert":
			_, err = ts.InboxRevert("01JZZZZZZZZZZZZZZZZZZZZZZZ")
			wantCode(t, "inbox revert of an unknown item", err, KindNotFound, "inbox.not_found")
			err = nil
		}
		if err != nil {
			t.Fatalf("inbox %s: %v", name, err)
		}
		if n := countContaining(ts.warns, "cannot open gone: "); n != 1 {
			t.Errorf("inbox %s warned %d times of gone: %q", name, n, ts.warns)
		}
		// the store --store names is an error
		ts.StoreName = "gone"
		ts.warns = nil
		if _, _, err := ts.InboxList(false); err == nil || !strings.Contains(err.Error(), "gone") {
			t.Errorf("inbox %s --store gone err = %v; want an error", name, err)
		}
	}
}

// Warn may call back into the session: nothing is held while it runs.
func TestWarnMayCallBackIntoTheSession(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	addGoneStore(t, ts, false)
	ts.Warn = func(m string) {
		ts.warns = append(ts.warns, m)
		if _, err := ts.Mount(); err != nil {
			t.Errorf("Mount inside Warn: %v", err)
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := ts.Mount(); err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Mount deadlocked with a Warn that calls Mount")
	}
	if n := countContaining(ts.warns, "cannot open gone"); n != 1 {
		t.Errorf("warnings = %q; want gone once", ts.warns)
	}
}

// An unknown store given to add is said as the other commands say it.
func TestAddHereUnknownStoreIsStoreNotFound(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	_, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Store: "nope"}, Task: AddInput{Title: "x"}})
	wantCode(t, "AddHere in nope", err, KindNotFound, "store.not_found")
}

// An ambiguity found by hh use --store names the input as the user typed it.
func TestUseWithStoreAmbiguityKeepsTheInput(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	s, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.ForStore("org-a").Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := ix.Workspaces()[0].ULID
	for _, title := range []string{"one", "two"} {
		task := &model.Task{ULID: model.NewULID(), Title: title, Kind: "project", Status: "todo", Workspace: ws, FormerIDs: []string{"OLD-1"}}
		if _, err := ts.ForStore("org-a").Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
			t.Fatal(err)
		}
	}
	ts.StoreName = "org-a"
	_, err = ts.Use("OLD-1")
	wantCode(t, "Use(OLD-1) --store org-a", err, KindAmbiguous, "task.ambiguous")
	if e := AsError(err); e == nil || e.Args["input"] != "OLD-1" || strings.Contains(err.Error(), "org-a:OLD-1") {
		t.Errorf("err = %v; want the input OLD-1 as typed", err)
	}
}

// A store whose inbox cannot be taken in is warned of as that, not as one
// that does not open.
func TestInboxStoreWhoseInboxCannotBeReadIsWarnedOf(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	s, err := ts.ForStore("org-b").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	// a directory where the items file is, so that taking in fails
	if err := os.MkdirAll(filepath.Join(s.Dir(), store.LocalDir, "inbox.jsonl"), 0o750); err != nil {
		t.Fatal(err)
	}
	stores, err := ts.InboxStores()
	if err != nil || len(stores) != 1 || stores[0].Name != "org-a" {
		t.Fatalf("InboxStores = %v, %v; want org-a only", stores, err)
	}
	if n := countContaining(ts.warns, "cannot read the inbox of org-b: "); n != 1 || countContaining(ts.warns, "cannot open") != 0 {
		t.Errorf("warnings = %q", ts.warns)
	}
}
