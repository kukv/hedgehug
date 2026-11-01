package ops

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// advance sets the clock of ts to t0 plus d, for the sessions made after it.
func advance(ts *testSession, d time.Duration) {
	base := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	now := base.Add(d)
	ts.Now = func() time.Time { return now }
}

func storeNames(es []string) string { return strings.Join(es, ",") }

func TestListAcrossStores(t *testing.T) {
	ts := newTwoStoreSession(t)
	l, err := ts.List(mustParse(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !l.Many || len(l.Stores) != 2 || len(l.Entries) != 2 {
		t.Fatalf("List = %d stores, many %v, %d entries", len(l.Stores), l.Many, len(l.Entries))
	}
	if l.Entries[0].Store.Name != "org-a" || l.Entries[1].Store.Name != "org-b" || l.Entries[1].Entry.Title != "org-b task" {
		t.Errorf("entries = %v %v", l.Entries[0].Store.Name, l.Entries[1].Store.Name)
	}
	one, err := ts.List(mustParse(t, "store:org-b"))
	if err != nil || !one.Many || len(one.Entries) != 1 || one.Entries[0].Store.Name != "org-b" {
		t.Errorf("store:org-b = %+v, %v", one, err)
	}
	_, err = ts.List(mustParse(t, "store:nope"))
	wantErr(t, err, KindUsage, "query.invalid")
	if err != nil && !strings.Contains(err.Error(), `unknown store "nope"`) {
		t.Errorf("err = %v", err)
	}
	// --store takes one store, and store: can only narrow it
	b := ts.ForStore("org-b")
	only, err := b.List(mustParse(t, ""))
	if err != nil || !only.Many || len(only.Entries) != 1 {
		t.Errorf("--store org-b = %+v, %v", only, err)
	}
	none, err := b.List(mustParse(t, "store:org-a"))
	if err != nil || len(none.Entries) != 0 {
		t.Errorf("--store org-b with store:org-a = %+v, %v", none, err)
	}
	_, err = ts.ForStore("nope").List(mustParse(t, ""))
	wantErr(t, err, KindNotFound, "store.not_found")
}

func TestListWarnsOfAStoreThatDoesNotOpen(t *testing.T) {
	ts := newTwoStoreSession(t)
	_, e, err := ts.ForStore("org-b").Entry()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(e.Path); err != nil {
		t.Fatal(err)
	}
	l, err := ts.List(mustParse(t, ""))
	if err != nil || l.Many || len(l.Entries) != 1 || l.Entries[0].Store.Name != "org-a" {
		t.Fatalf("List = %+v, %v; want org-a's only", l, err)
	}
	if len(ts.warns) != 1 || !strings.Contains(ts.warns[0], "org-b") {
		t.Errorf("warnings = %q; want one about org-b", ts.warns)
	}
	// asking again does not warn again
	if _, err := ts.List(mustParse(t, "")); err != nil || len(ts.warns) != 1 {
		t.Errorf("second List: %v, warnings %q; want still one", err, ts.warns)
	}
	// the one --store names is an error
	_, err = ts.ForStore("org-b").List(mustParse(t, ""))
	wantErr(t, err, KindInvalid, "store.open")
}

func TestTreePerStore(t *testing.T) {
	ts := newTwoStoreSession(t)
	if _, err := ts.ForStore("org-a").Add(AddInput{Title: "child", Parent: "API-1"}); err != nil {
		t.Fatal(err)
	}
	tr, err := ts.Tree(mustParse(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !tr.Many || len(tr.Stores) != 2 {
		t.Fatalf("Tree = %d stores, many %v", len(tr.Stores), tr.Many)
	}
	a, b := tr.Stores[0], tr.Stores[1]
	if a.Store.Name != "org-a" || b.Store.Name != "org-b" || a.Index.StoreName() != "org-a" {
		t.Errorf("stores = %s %s", a.Store.Name, b.Store.Name)
	}
	if got := ids(Flatten(a.Roots)); !slices.Equal(got, []string{"API-1", "API-2"}) {
		t.Errorf("org-a tree = %v", got)
	}
	if got := ids(Flatten(b.Roots)); !slices.Equal(got, []string{"WEB-1"}) {
		t.Errorf("org-b tree = %v", got)
	}
	// a store with no match stays, with nothing in it
	tr, err = ts.Tree(mustParse(t, "child"))
	if err != nil || len(tr.Stores) != 2 || len(tr.Stores[1].Roots) != 0 || len(tr.Stores[0].Roots) != 1 {
		t.Errorf("Tree(child) = %+v, %v", tr, err)
	}
	// store: leaves one, though two are mounted
	tr, err = ts.Tree(mustParse(t, "store:org-b"))
	if err != nil || !tr.Many || len(tr.Stores) != 1 {
		t.Errorf("Tree(store:org-b) = %+v, %v", tr, err)
	}
	_, err = ts.Tree(mustParse(t, "sort:title"))
	wantErr(t, err, KindUsage, "query.tree_order")
}

func TestLogMixesStoresByTime(t *testing.T) {
	ts := newTwoStoreSession(t)
	advance(ts, time.Hour)
	if _, err := ts.ForStore("org-b").Add(AddInput{Title: "b+1"}); err != nil {
		t.Fatal(err)
	}
	advance(ts, 2*time.Hour)
	if _, err := ts.ForStore("org-a").Add(AddInput{Title: "a+2"}); err != nil {
		t.Fatal(err)
	}
	advance(ts, 3*time.Hour)
	if _, err := ts.ForStore("org-b").Add(AddInput{Title: "b+3"}); err != nil {
		t.Fatal(err)
	}
	lr, err := ts.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !lr.Many {
		t.Error("Many = false with two stores")
	}
	var got []string
	for _, l := range lr.Logs {
		got = append(got, l.Store)
		if l.Index == nil || l.Index.StoreName() != l.Store {
			t.Errorf("commit of %s has the index of %v", l.Store, l.Index)
		}
	}
	// newest first; the same time in the order of the stores (org-a's first)
	if prefix := "org-b,org-a,org-b"; !strings.HasPrefix(storeNames(got), prefix) {
		t.Errorf("stores = %s; want %s first", storeNames(got), prefix)
	}
	for i := 1; i < len(lr.Logs); i++ {
		if lr.Logs[i].Entry.When.After(lr.Logs[i-1].Entry.When) {
			t.Fatalf("commit %d is newer than %d", i, i-1)
		}
	}
	// the commits of the same time (the creation and the first task of each
	// store): org-a's before org-b's
	last := lr.Logs[len(lr.Logs)-4:]
	if got := last[0].Store + last[1].Store + last[2].Store + last[3].Store; got != "org-aorg-aorg-borg-b" {
		t.Errorf("oldest commits are of %s; want org-a twice, then org-b twice", got)
	}
	// the limit is for all of them
	two, err := ts.Log(nil, 2)
	if err != nil || len(two.Logs) != 2 || two.Logs[0].Store != "org-b" || two.Logs[1].Store != "org-a" {
		t.Errorf("Log(nil, 2) = %+v, %v", two.Logs, err)
	}
	// a commit tells its store in --json
	views := StoreLogViews(two.Logs)
	if views[0].Store != "org-b" || views[1].Store != "org-a" {
		t.Errorf("views = %+v", views)
	}
	// one task: in its store, and the log is not "many"
	one, err := ts.Log([]string{"WEB-1"}, 0)
	if err != nil || one.Many || len(one.Logs) == 0 || one.Logs[0].Store != "org-b" {
		t.Errorf("Log(WEB-1) = %+v, %v", one, err)
	}
	// --store takes the commits of that one
	onlyA, err := ts.ForStore("org-a").Log(nil, 0)
	if err != nil || !onlyA.Many {
		t.Fatalf("--store org-a = %+v, %v", onlyA, err)
	}
	for _, l := range onlyA.Logs {
		if l.Store != "org-a" {
			t.Errorf("--store org-a has a commit of %s", l.Store)
		}
	}
}

func TestViewsTellTheirStore(t *testing.T) {
	ts := newTwoStoreSession(t)
	r, err := ts.Show("WEB-1")
	if err != nil || r.Store != "org-b" {
		t.Fatalf("Show(WEB-1).Store = %q, %v", r.Store, err)
	}
	if v := TaskViewOf(r.Index, r.Entry); v.Store != "org-b" {
		t.Errorf("TaskViewOf.Store = %q", v.Store)
	}
	if v := ShowViewOf(r); v.Store != "org-b" || len(v.History) == 0 || v.History[0].Store != "org-b" {
		t.Errorf("ShowViewOf = %q, history %+v", v.Store, v.History)
	}
	one := newTestSession(t)
	if _, err := one.Add(AddInput{Title: "x"}); err != nil {
		t.Fatal(err)
	}
	if r, err := one.Show("ME-1"); err != nil || r.Store != "me" {
		t.Errorf("Show(ME-1).Store = %q, %v", r.Store, err)
	}
}

func TestListByAnIDOfOneStore(t *testing.T) {
	ts := newTwoStoreSession(t)
	if _, err := ts.ForStore("org-a").Add(AddInput{Title: "child", Parent: "API-1"}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"parent:API-1", "parent:org-a:API-1"} {
		l, err := ts.List(mustParse(t, q))
		if err != nil || len(l.Entries) != 1 || l.Entries[0].Entry.Title != "child" {
			t.Errorf("%s = %+v, %v", q, l.Entries, err)
		}
	}
	// org-b's WEB-1 is not in org-a, and written for org-a it is nowhere
	l, err := ts.List(mustParse(t, "parent:org-b:WEB-1"))
	if err != nil || len(l.Entries) != 0 {
		t.Errorf("parent:org-b:WEB-1 = %+v, %v; want nothing, from org-b", l.Entries, err)
	}
	for _, q := range []string{"parent:ZZ-9", "parent:org-a:WEB-1", "parent:org-c:API-1"} {
		_, err := ts.List(mustParse(t, q))
		wantErr(t, err, KindUsage, "query.invalid")
	}
}
