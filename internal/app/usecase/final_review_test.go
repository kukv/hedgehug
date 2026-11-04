package usecase_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/query"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// addGoneStore sets up a store called gone in config.yaml at a path where
// there is no store, with a remote when withRemote.
func addGoneStore(t *testing.T, ts *usecasetest.Session, withRemote bool) {
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
	if err := c.SetStoreAccount("gone", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.DropMount()
	ts.Warns = nil
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
	ts := usecasetest.NewTwoStoreWith(t, "API", "API")
	idA := usecasetest.AddProjectIn(t, ts, "org-a", "Release A")
	idB := usecasetest.AddProjectIn(t, ts, "org-b", "Release B")
	if idA != idB {
		t.Fatalf("ids %s and %s; want the same id in both stores", idA, idB)
	}
	// without --store the id is in both
	if _, err := stores.Use(ts.Session, idA); usecase.KindOf(err) != usecase.KindAmbiguous {
		t.Fatalf("Use(%s) err = %v; want ambiguous", idA, err)
	}
	ts.StoreName = "org-b"
	r, err := stores.Use(ts.Session, idA)
	if err != nil || r.Store != "org-b" || r.Project.Title != "Release B" {
		t.Fatalf("Use(%s) --store org-b = %+v, %v; want org-b's project", idA, r, err)
	}
	// the same store spelled in the id is fine
	if r, err = stores.Use(ts.Session, "org-b:"+idA); err != nil || r.Store != "org-b" {
		t.Errorf("Use(org-b:%s) --store org-b = %+v, %v", idA, r, err)
	}
	// another store's name is a usage error
	_, err = stores.Use(ts.Session, "org-a:"+idA)
	usecasetest.WantCode(t, "Use(org-a:...) --store org-b", err, usecase.KindUsage, "store.conflict")
	// a project that is only in the other store is not found
	ts.StoreName = "org-a"
	other := usecasetest.AddProjectIn(t, ts, "org-b", "Only B")
	_, err = stores.Use(ts.Session, other)
	usecasetest.WantCode(t, "Use(project of org-b) --store org-a", err, usecase.KindNotFound, "task.not_found")
	ts.StoreName = "nope"
	_, err = stores.Use(ts.Session, idA)
	usecasetest.WantCode(t, "Use --store nope", err, usecase.KindNotFound, "store.not_found")
}

// A command about one store does not warn of a store it is not about.
func TestStoreNameKeepsTheWarningsOfOtherStoresQuiet(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	addGoneStore(t, ts, false)
	ts.StoreName = "org-a"
	if _, err := browse.List(ts.Session, &query.Query{}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Mount(); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(ts.Warns, "cannot open gone"); n != 0 {
		t.Errorf("--store org-a warned %d times of gone: %q", n, ts.Warns)
	}
	// without --store it still does, once
	ts.StoreName = ""
	if _, err := browse.List(ts.Session, &query.Query{}); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(ts.Warns, "cannot open gone"); n != 1 {
		t.Errorf("List warned %d times of gone: %q", n, ts.Warns)
	}
}

// A sync that fails for a store that does not open says so in its report;
// the cross-store check of the other stores does not say it again.
func TestSyncDoesNotWarnTwiceOfAStoreItReports(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	addGoneStore(t, ts, true)
	rs, err := replication.Sync(ts.Session, true, true)
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
	if n := countContaining(ts.Warns, "cannot open gone"); n != 0 {
		t.Errorf("warnings %q repeat what the report of gone says", ts.Warns)
	}
	// the silence was for the sync only
	ts.Warns = nil
	if _, err := browse.List(ts.Session, &query.Query{}); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(ts.Warns, "cannot open gone"); n != 1 {
		t.Errorf("List after the sync warned %d times of gone: %q", n, ts.Warns)
	}
}

// inbox leaves out a store that does not open, with a warning, but not the
// one --store names.
func TestInboxSkipsAStoreThatDoesNotOpen(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"list", "read", "revert"} {
		ts := usecasetest.NewTwoStore(t)
		addGoneStore(t, ts, false)
		var err error
		switch name {
		case "list":
			_, _, err = notifications.InboxList(ts.Session, false)
		case "read":
			_, err = notifications.InboxRead(ts.Session, nil, true)
		case "revert":
			_, err = notifications.InboxRevert(ts.Session, "01JZZZZZZZZZZZZZZZZZZZZZZZ")
			usecasetest.WantCode(t, "inbox revert of an unknown item", err, usecase.KindNotFound, "inbox.not_found")
			err = nil
		}
		if err != nil {
			t.Fatalf("inbox %s: %v", name, err)
		}
		if n := countContaining(ts.Warns, "cannot open gone: "); n != 1 {
			t.Errorf("inbox %s warned %d times of gone: %q", name, n, ts.Warns)
		}
		// the store --store names is an error
		ts.StoreName = "gone"
		ts.Warns = nil
		if _, _, err := notifications.InboxList(ts.Session, false); err == nil || !strings.Contains(err.Error(), "gone") {
			t.Errorf("inbox %s --store gone err = %v; want an error", name, err)
		}
	}
}

// Warn may call back into the session: nothing is held while it runs.
func TestWarnMayCallBackIntoTheSession(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	addGoneStore(t, ts, false)
	ts.Warn = func(m string) {
		ts.Warns = append(ts.Warns, m)
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
	if n := countContaining(ts.Warns, "cannot open gone"); n != 1 {
		t.Errorf("warnings = %q; want gone once", ts.Warns)
	}
}

// An unknown store given to add is said as the other commands say it.
func TestAddHereUnknownStoreIsStoreNotFound(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	_, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Store: "nope"}, Task: tasks.AddInput{Title: "x"}})
	usecasetest.WantCode(t, "AddHere in nope", err, usecase.KindNotFound, "store.not_found")
}

// An ambiguity found by hh use --store names the input as the user typed it.
func TestUseWithStoreAmbiguityKeepsTheInput(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
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
	_, err = stores.Use(ts.Session, "OLD-1")
	usecasetest.WantCode(t, "Use(OLD-1) --store org-a", err, usecase.KindAmbiguous, "task.ambiguous")
	if e := usecase.AsError(err); e == nil || e.Args["input"] != "OLD-1" || strings.Contains(err.Error(), "org-a:OLD-1") {
		t.Errorf("err = %v; want the input OLD-1 as typed", err)
	}
}

// A store whose inbox cannot be taken in is warned of as that, not as one
// that does not open.
func TestInboxStoreWhoseInboxCannotBeReadIsWarnedOf(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	s, err := ts.ForStore("org-b").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	// a directory where the items file is, so that taking in fails
	if err := os.MkdirAll(filepath.Join(s.Dir(), store.LocalDir, "inbox.jsonl"), 0o750); err != nil {
		t.Fatal(err)
	}
	stores, err := notifications.InboxStores(ts.Session)
	if err != nil || len(stores) != 1 || stores[0].Name != "org-a" {
		t.Fatalf("InboxStores = %v, %v; want org-a only", stores, err)
	}
	if n := countContaining(ts.Warns, "cannot read the inbox of org-b: "); n != 1 || countContaining(ts.Warns, "cannot open") != 0 {
		t.Errorf("warnings = %q", ts.Warns)
	}
}
