package browse_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
)

// advance sets the clock of ts to t0 plus d, for the sessions made after it.
func advance(ts *usecasetest.Session, d time.Duration) {
	base := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	now := base.Add(d)
	ts.Now = func() time.Time { return now }
}

func storeNames(es []string) string { return strings.Join(es, ",") }

func TestListAcrossStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	l, err := browse.List(ts.Session, "")
	if err != nil {
		t.Fatal(err)
	}
	if !l.Many || len(l.Tasks) != 2 {
		t.Fatalf("List = many %v, %d tasks", l.Many, len(l.Tasks))
	}
	if l.Tasks[0].Store != "org-a" || l.Tasks[1].Store != "org-b" || l.Tasks[1].Title != "org-b task" {
		t.Errorf("tasks = %v %v", l.Tasks[0].Store, l.Tasks[1].Store)
	}
	one, err := browse.List(ts.Session, "store:org-b")
	if err != nil || !one.Many || len(one.Tasks) != 1 || one.Tasks[0].Store != "org-b" {
		t.Errorf("store:org-b = %+v, %v", one, err)
	}
	_, err = browse.List(ts.Session, "store:nope")
	usecasetest.WantErr(t, err, usecase.KindUsage, "query.invalid")
	if err != nil && !strings.Contains(err.Error(), `unknown store "nope"`) {
		t.Errorf("err = %v", err)
	}
	// --store takes one store, and store: can only narrow it
	b := ts.ForStore("org-b")
	only, err := browse.List(b, "")
	if err != nil || !only.Many || len(only.Tasks) != 1 {
		t.Errorf("--store org-b = %+v, %v", only, err)
	}
	none, err := browse.List(b, "store:org-a")
	if err != nil || len(none.Tasks) != 0 {
		t.Errorf("--store org-b with store:org-a = %+v, %v", none, err)
	}
	_, err = browse.List(ts.ForStore("nope"), "")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "store.not_found")
}

func TestListWarnsOfAStoreThatDoesNotOpen(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	_, e, err := ts.ForStore("org-b").Entry()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(e.Path); err != nil {
		t.Fatal(err)
	}
	l, err := browse.List(ts.Session, "")
	if err != nil || l.Many || len(l.Tasks) != 1 || l.Tasks[0].Store != "org-a" {
		t.Fatalf("List = %+v, %v; want org-a's only", l, err)
	}
	if len(ts.Warns) != 1 || !strings.Contains(ts.Warns[0], "org-b") {
		t.Errorf("warnings = %q; want one about org-b", ts.Warns)
	}
	// asking again does not warn again
	if _, err := browse.List(ts.Session, ""); err != nil || len(ts.Warns) != 1 {
		t.Errorf("second List: %v, warnings %q; want still one", err, ts.Warns)
	}
	// the one --store names is an error
	_, err = browse.List(ts.ForStore("org-b"), "")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "store.open")
}

func TestTreePerStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	if _, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "child", Parent: "API-1"}); err != nil {
		t.Fatal(err)
	}
	tr, err := browse.Tree(ts.Session, "")
	if err != nil {
		t.Fatal(err)
	}
	if !tr.Many || len(tr.Stores) != 2 {
		t.Fatalf("Tree = %d stores, many %v", len(tr.Stores), tr.Many)
	}
	a, b := tr.Stores[0], tr.Stores[1]
	if a.Store != "org-a" || b.Store != "org-b" {
		t.Errorf("stores = %s %s", a.Store, b.Store)
	}
	if got := ids(a.Drawn()); !slices.Equal(got, []string{"API-1", "API-2"}) {
		t.Errorf("org-a tree = %v", got)
	}
	if got := ids(b.Drawn()); !slices.Equal(got, []string{"WEB-1"}) {
		t.Errorf("org-b tree = %v", got)
	}
	// a store with no match stays, with nothing in it
	tr, err = browse.Tree(ts.Session, "child")
	if err != nil || len(tr.Stores) != 2 || len(tr.Stores[1].Roots) != 0 || len(tr.Stores[0].Roots) != 1 {
		t.Errorf("Tree(child) = %+v, %v", tr, err)
	}
	// store: leaves one, though two are mounted
	tr, err = browse.Tree(ts.Session, "store:org-b")
	if err != nil || !tr.Many || len(tr.Stores) != 1 {
		t.Errorf("Tree(store:org-b) = %+v, %v", tr, err)
	}
	_, err = browse.Tree(ts.Session, "sort:title")
	usecasetest.WantErr(t, err, usecase.KindUsage, "query.tree_order")
}

func TestLogMixesStoresByTime(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	advance(ts, time.Hour)
	if _, err := tasks.Add(ts.ForStore("org-b"), tasks.AddInput{Title: "b+1"}); err != nil {
		t.Fatal(err)
	}
	advance(ts, 2*time.Hour)
	if _, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "a+2"}); err != nil {
		t.Fatal(err)
	}
	advance(ts, 3*time.Hour)
	if _, err := tasks.Add(ts.ForStore("org-b"), tasks.AddInput{Title: "b+3"}); err != nil {
		t.Fatal(err)
	}
	lr, err := browse.Log(ts.Session, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !lr.Many {
		t.Error("Many = false with two stores")
	}
	var got []string
	for _, l := range lr.Logs {
		got = append(got, l.Store)
	}
	// newest first; the same time in the order of the stores (org-a's first)
	if prefix := "org-b,org-a,org-b"; !strings.HasPrefix(storeNames(got), prefix) {
		t.Errorf("stores = %s; want %s first", storeNames(got), prefix)
	}
	for i := 1; i < len(lr.Logs); i++ {
		if lr.Logs[i].Date.After(lr.Logs[i-1].Date) {
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
	two, err := browse.Log(ts.Session, nil, 2)
	if err != nil || len(two.Logs) != 2 || two.Logs[0].Store != "org-b" || two.Logs[1].Store != "org-a" {
		t.Errorf("Log(nil, 2) = %+v, %v", two.Logs, err)
	}
	// one task: in its store, and the log is not "many"
	one, err := browse.Log(ts.Session, []string{"WEB-1"}, 0)
	if err != nil || one.Many || len(one.Logs) == 0 || one.Logs[0].Store != "org-b" {
		t.Errorf("Log(WEB-1) = %+v, %v", one, err)
	}
	// --store takes the commits of that one
	onlyA, err := browse.Log(ts.ForStore("org-a"), nil, 0)
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
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	r, err := browse.Show(ts.Session, "WEB-1")
	if err != nil || r.Store != "org-b" {
		t.Fatalf("Show(WEB-1).Store = %q, %v", r.Store, err)
	}
	if len(r.History) == 0 || r.History[0].Store != "org-b" {
		t.Errorf("Show history = %+v", r.History)
	}
	one := usecasetest.New(t)
	if _, err := tasks.Add(one.Session, tasks.AddInput{Title: "x"}); err != nil {
		t.Fatal(err)
	}
	if r, err := browse.Show(one.Session, "ME-1"); err != nil || r.Store != "me" {
		t.Errorf("Show(ME-1).Store = %q, %v", r.Store, err)
	}
}

func TestListByAnIDOfOneStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	if _, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "child", Parent: "API-1"}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"parent:API-1", "parent:org-a:API-1"} {
		l, err := browse.List(ts.Session, q)
		if err != nil || len(l.Tasks) != 1 || l.Tasks[0].Title != "child" {
			t.Errorf("%s = %+v, %v", q, l.Tasks, err)
		}
	}
	// org-b's WEB-1 is not in org-a, and written for org-a it is nowhere
	l, err := browse.List(ts.Session, "parent:org-b:WEB-1")
	if err != nil || len(l.Tasks) != 0 {
		t.Errorf("parent:org-b:WEB-1 = %+v, %v; want nothing, from org-b", l.Tasks, err)
	}
	for _, q := range []string{"parent:ZZ-9", "parent:org-a:WEB-1", "parent:org-c:API-1"} {
		_, err := browse.List(ts.Session, q)
		usecasetest.WantErr(t, err, usecase.KindUsage, "query.invalid")
	}
}

func TestWorkspaceQueryAcrossStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	// org-a has no workspace org-b: it matches nothing there, not an error
	r, err := browse.List(ts.Session, "workspace:org-b")
	if err != nil || len(r.Tasks) != 1 || r.Tasks[0].Store != "org-b" {
		t.Fatalf("List(workspace:org-b) = %+v, %v", r.Tasks, err)
	}
	r, err = browse.List(ts.Session, "workspace:org-a,org-b")
	if err != nil || len(r.Tasks) != 2 {
		t.Errorf("List(workspace:org-a,org-b) = %d tasks, %v", len(r.Tasks), err)
	}
	// a workspace no store has is still an error
	_, err = browse.List(ts.Session, "workspace:nope")
	usecasetest.WantErr(t, err, usecase.KindUsage, "query.invalid")
}

func TestLogInWorkspaces(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	a := ts.ForStore("org-a")
	advance(ts, time.Hour)
	if _, err := workspaces.WorkspaceAdd(a, "docs", "DOC"); err != nil {
		t.Fatal(err)
	}
	advance(ts, time.Hour)
	if _, err := tasks.Add(a, tasks.AddInput{Title: "guide", Workspace: "docs"}); err != nil {
		t.Fatal(err)
	}
	wsOf := func(l usecase.LogView) string {
		if l.Workspace == nil {
			return ""
		}
		return l.Workspace.Name
	}
	// the workspace's commit by its trailer, the task's by where it is
	lr, err := browse.LogIn(ts.Session, nil, []string{"DOC"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, l := range lr.Logs {
		if wsOf(l) != "docs" {
			t.Errorf("a commit of %q in the log of docs", wsOf(l))
		}
		op, _ := l.Trailer("Hedgehug-Op")
		names = append(names, op)
	}
	if strings.Join(names, ",") != "add,workspace" {
		t.Errorf("ops = %v; want add, workspace", names)
	}
	// a name in one store and a name in the other
	lr, err = browse.LogIn(ts.Session, nil, []string{"org-b", "docs"}, 0)
	if err != nil || len(lr.Logs) != 3 {
		t.Fatalf("LogIn(org-b, docs) = %d, %v; want docs' two and org-b task's add", len(lr.Logs), err)
	}
	if wsOf(lr.Logs[2]) != "org-b" {
		t.Errorf("oldest = %q; want org-b", wsOf(lr.Logs[2]))
	}
	// the limit counts what is kept
	one, err := browse.LogIn(ts.Session, nil, []string{"docs"}, 1)
	if err != nil || len(one.Logs) != 1 || wsOf(one.Logs[0]) != "docs" {
		t.Errorf("LogIn(docs, 1) = %+v, %v", one.Logs, err)
	}
	_, err = browse.LogIn(ts.Session, nil, []string{"nope"}, 0)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "workspace.not_found")
	_, err = browse.LogIn(ts.Session, []string{"WEB-1"}, []string{"org-b"}, 0)
	usecasetest.WantErr(t, err, usecase.KindUsage, "log.workspace_with_task")
	// every commit of a task tells its workspace
	all, err := browse.Log(ts.Session, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range all.Logs {
		if v.Task != nil && (v.Workspace == nil || v.Workspace.ULID == "" || v.Workspace.IDPrefix == "") {
			t.Errorf("commit %s of task %s has workspace %+v", v.Hash[:7], *v.Task, v.Workspace)
		}
	}
}
