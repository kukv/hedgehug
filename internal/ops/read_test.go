package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/query"
)

func mustParse(t *testing.T, s string) *query.Query {
	t.Helper()
	q, err := ParseQuery(s)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func ids(es []*TreeNode) []string {
	var out []string
	for _, n := range es {
		out = append(out, n.Entry.ID)
	}
	return out
}

func TestListTreeShowLog(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "p"}, {Title: "c", Parent: "ME-1"}, {Title: "other"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	q := mustParse(t, "c")
	l, err := ts.List(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) != 1 || l.Entries[0].Entry.ID != "ME-2" || l.Entries[0].Store.Name != "me" || l.Many || len(l.Stores) != 1 {
		t.Fatalf("List = %+v", l)
	}
	tr, err := ts.Tree(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Stores) != 1 || tr.Many {
		t.Fatalf("Tree has %d stores, many %v; want one", len(tr.Stores), tr.Many)
	}
	roots := tr.Stores[0].Roots
	if len(roots) != 1 || roots[0].Entry.ID != "ME-1" || !roots[0].Context {
		t.Fatalf("Tree roots = %+v; want ME-1 as context", roots)
	}
	if c := roots[0].Children; len(c) != 1 || c[0].Entry.ID != "ME-2" || c[0].Context {
		t.Fatalf("Tree children = %+v", c)
	}
	sh, err := ts.Show("ME-2")
	if err != nil {
		t.Fatal(err)
	}
	if sh.Task.Title != "c" || len(sh.History) != 1 {
		t.Errorf("Show = %q with %d commits; want c with 1", sh.Task.Title, len(sh.History))
	}
	lr, err := ts.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.Logs) < 3 || lr.Many {
		t.Errorf("Log has %d commits, many %v; want at least 3 of one store", len(lr.Logs), lr.Many)
	}
}

func TestParseQuery(t *testing.T) {
	q, err := ParseQuery("status:todo group:status")
	if err != nil || len(q.Terms) != 2 {
		t.Fatalf("ParseQuery = %+v, %v; want two terms", q, err)
	}
	_, err = ParseQuery(`title:"unclosed`)
	wantErr(t, err, KindUsage, "query.invalid")
}

func TestListGroupsAndErrors(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "a"}, {Title: "b", Status: "done"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	q := mustParse(t, "group:status sort:title")
	l, err := ts.List(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Groups) != 2 || len(l.Entries) != 2 {
		t.Fatalf("List = %d groups, %d entries", len(l.Groups), len(l.Entries))
	}
	if l.Entries[0].Entry.Title != l.Groups[0].Entries[0].Entry.Title || l.Entries[1].Entry.Title != l.Groups[1].Entries[0].Entry.Title {
		t.Errorf("Entries are not the groups' tasks in order: %v", l.Entries)
	}
	// A key the schema does not have parses, and fails when it runs.
	_, err = ts.List(mustParse(t, "nope:x"))
	wantErr(t, err, KindUsage, "query.invalid")
}

func TestHasTerm(t *testing.T) {
	q := mustParse(t, "status:todo sort:title")
	if !HasTerm(q, "sort") || !HasTerm(q, "status") || HasTerm(q, "group") {
		t.Errorf("HasTerm wrong for %+v", q.Terms)
	}
	if HasTerm(&query.Query{}, "sort") {
		t.Error("HasTerm on an empty query = true")
	}
}

func TestTreeRefusesGroupAndSort(t *testing.T) {
	ts := newTestSession(t)
	for _, s := range []string{"group:status", "sort:title"} {
		_, err := ts.Tree(mustParse(t, s))
		wantErr(t, err, KindUsage, "query.tree_order")
		if want := "--tree orders tasks by rank; it takes neither group: nor sort:"; err != nil && err.Error() != want {
			t.Errorf("%s: message = %q; want %q", s, err, want)
		}
	}
}

func TestTreeViewOfAndFlatten(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "p"}, {Title: "c1", Parent: "ME-1"}, {Title: "g", Parent: "ME-2"}, {Title: "c2", Parent: "ME-1"}, {Title: "q"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	tr, err := ts.Tree(mustParse(t, "g"))
	if err != nil {
		t.Fatal(err)
	}
	// Only the ancestors of g are drawn, c2 and q are not.
	if got := ids(Flatten(tr.Stores[0].Roots)); !slices.Equal(got, []string{"ME-1", "ME-2", "ME-3"}) {
		t.Fatalf("Flatten = %v; want ME-1 ME-2 ME-3", got)
	}
	if len(tr.Stores[0].Cycle) != 0 {
		t.Errorf("Cycle = %v; want none", ids(tr.Stores[0].Cycle))
	}
	v := TreeViewOf(tr.Stores[0].Index, tr.Stores[0].Roots[0])
	if v.ID != "ME-1" || !v.Context || v.InCycle || len(v.Children) != 1 {
		t.Fatalf("TreeViewOf root = %+v", v)
	}
	mid := v.Children[0]
	if mid.ID != "ME-2" || !mid.Context || len(mid.Children) != 1 {
		t.Fatalf("TreeViewOf middle = %+v", mid)
	}
	if leaf := mid.Children[0]; leaf.ID != "ME-3" || leaf.Context || len(leaf.Children) != 0 {
		t.Errorf("TreeViewOf leaf = %+v", leaf)
	}
	// The whole forest: two roots, in rank order, depth first.
	all, err := ts.Tree(mustParse(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(Flatten(all.Stores[0].Roots)); !slices.Equal(got, []string{"ME-1", "ME-2", "ME-3", "ME-4", "ME-5"}) {
		t.Errorf("Flatten(all) = %v", got)
	}
	if Flatten(nil) != nil {
		t.Error("Flatten(nil) is not nil")
	}
}

// rawTask commits a task file past the store's checks, as a sync or a hand
// edit could.
func rawTask(t *testing.T, ts *testSession, id string, ulid model.ULID, front string) {
	t.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.TaskPath(ulid)
	p := filepath.Join(e.Path, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	body := "---\nulid: " + ulid.String() + "\nid: " + id + "\nworkspace: " + ix.Workspaces()[0].ULID.String() + "\nkind: task\nstatus: todo\n" + front + "---\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func TestTreeParentCycle(t *testing.T) {
	ts := newTestSession(t)
	x, y, c := model.NewULID(), model.NewULID(), model.NewULID()
	rawTask(t, ts, "ME-1", x, "title: X\nparent: "+y.String()+"\n")
	rawTask(t, ts, "ME-2", y, "title: Y\nparent: "+x.String()+"\n")
	rawTask(t, ts, "ME-3", c, "title: C\nparent: "+x.String()+"\n")
	tr, err := ts.Tree(mustParse(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	ts0 := tr.Stores[0]
	if len(ts0.Roots) != 0 || len(ts0.Cycle) != 2 {
		t.Fatalf("Roots = %v, Cycle = %v; want no roots and the two members", ids(ts0.Roots), ids(ts0.Cycle))
	}
	for _, n := range ts0.Cycle {
		if !n.InCycle {
			t.Errorf("%s is not marked InCycle", n.Entry.ID)
		}
	}
	// C hangs under exactly one member and is not itself in the cycle.
	var under *TreeNode
	for _, n := range ts0.Cycle {
		for _, ch := range n.Children {
			if ch.Entry.ID == "ME-3" {
				under = ch
			}
		}
	}
	if under == nil || under.InCycle {
		t.Fatalf("C is not drawn under a cycle member, or is marked InCycle: %+v", under)
	}
	v := TreeViewOf(ts0.Index, ts0.Cycle[0])
	if !v.InCycle {
		t.Errorf("TreeViewOf(cycle[0]).InCycle = false")
	}
}

func TestShowFullTask(t *testing.T) {
	ts := newTestSession(t)
	a := ts.addTask("a", func(t *model.Task) { t.Body = "the body\n"; t.Watchers = []string{"bob"}; t.Repos = []string{"r1"} })
	b := ts.addTask("b", nil)
	c := ts.addTask("c", nil)
	// b is blocked by a; c duplicates a; c writes a as #ME-1 in its body.
	if _, err := ts.Dep("ME-2", "ME-1", true); err != nil {
		t.Fatal(err)
	}
	as, err := ParseAssigns([]string{"duplicates+=ME-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Set("ME-3", as); err != nil {
		t.Fatal(err)
	}
	_ = ts.addTask("d", func(t *model.Task) { t.Body = "see #ME-1\n" })
	if _, err := ts.CommentAdd("ME-1", "first"); err != nil {
		t.Fatal(err)
	}
	gone, err := ts.CommentAdd("ME-1", "second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.CommentAdd("ME-1", "third"); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.CommentRemove(gone.Comment.ULID.String()); err != nil {
		t.Fatal(err)
	}

	r, err := ts.Show("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Entry.ULID != a.ULID || r.Task.Body != "the body\n" {
		t.Fatalf("Show = %v with body %q", r.Entry.ID, r.Task.Body)
	}
	var cs []string
	for _, cm := range r.Comments {
		cs = append(cs, cm.Body)
	}
	if !slices.Equal(cs, []string{"first\n", "third\n"}) {
		t.Errorf("Comments = %q; want first and third (the removed one is left out)", cs)
	}
	if len(r.Relations) != 2 || r.Relations[0].Key != "blocks" || r.Relations[0].Label != "blocks" || r.Relations[0].Tasks[0].ULID != b.ULID {
		t.Fatalf("Relations = %+v", r.Relations)
	}
	if rel := r.Relations[1]; rel.Key != "duplicates" || rel.Label != "重複されている" || len(rel.Tasks) != 1 || rel.Tasks[0].ULID != c.ULID {
		t.Errorf("Relations[1] = %+v; want duplicates / 重複されている / c", rel)
	}
	if len(r.Back) != 1 || r.Back[0].Title != "d" {
		t.Errorf("Back = %v; want d", r.Back)
	}
	if len(r.History) == 0 || len(r.History) > ShowHistory {
		t.Errorf("History has %d commits; want 1 to %d", len(r.History), ShowHistory)
	}

	v := ShowViewOf(r)
	if v.ID != "ME-1" || v.Body != "the body\n" || !slices.Equal(v.Watchers, []string{"bob"}) || !slices.Equal(v.Repos, []string{"r1"}) {
		t.Errorf("ShowViewOf = %+v", v)
	}
	if len(v.Comments) != 2 || v.Comments[0].Body != "first\n" || v.Comments[0].ULID != r.Comments[0].ULID.String() {
		t.Errorf("Comments view = %+v", v.Comments)
	}
	if got := v.Reverse["blocks"]; len(got) != 1 || got[0].ID != "ME-2" || got[0].Title != "b" || got[0].ULID != b.ULID.String() {
		t.Errorf("Reverse[blocks] = %+v", got)
	}
	if got := v.Reverse["duplicates"]; len(got) != 1 || got[0].ID != "ME-3" {
		t.Errorf("Reverse[duplicates] = %+v", got)
	}
	if len(v.ReferencedBy) != 1 || v.ReferencedBy[0].ID != "ME-4" {
		t.Errorf("ReferencedBy = %+v", v.ReferencedBy)
	}
	if len(v.History) != len(r.History) || v.History[0].ID == nil || *v.History[0].ID != "ME-1" {
		t.Errorf("History view = %+v", v.History)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"body":`, `"watchers":`, `"repos":`, `"comments":`, `"reverse":`, `"referenced_by":`, `"history":`, `"title":"a"`} {
		if !strings.Contains(string(out), key) {
			t.Errorf("JSON lacks %s: %s", key, out)
		}
	}
}

func TestShowViewOfEmpty(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "bare"}); err != nil {
		t.Fatal(err)
	}
	r, err := ts.Show("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Comments) != 0 || len(r.Relations) != 0 || len(r.Back) != 0 {
		t.Errorf("Show = %d comments, %d relations, %d back; want none", len(r.Comments), len(r.Relations), len(r.Back))
	}
	v := ShowViewOf(r)
	if v.Watchers == nil || v.Repos == nil || v.Comments == nil || v.Reverse == nil || v.ReferencedBy == nil {
		t.Errorf("empty fields must be empty, not nil: %+v", v)
	}
	out, _ := json.Marshal(v)
	for _, want := range []string{`"watchers":[]`, `"repos":[]`, `"comments":[]`, `"reverse":{}`, `"referenced_by":[]`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("JSON lacks %s: %s", want, out)
		}
	}
}

func TestShowBrokenTask(t *testing.T) {
	ts := newTestSession(t)
	ulid := model.NewULID()
	rawTask(t, ts, "ME-1", ulid, "title: [unclosed\n")
	r, err := ts.Show(ulid.String())
	if err != nil {
		t.Fatal(err)
	}
	if r.Entry.Broken == "" || r.Task != nil || r.Comments != nil {
		t.Fatalf("Show = broken %q, task %v, comments %v; want a broken entry with no task", r.Entry.Broken, r.Task, r.Comments)
	}
	v := ShowViewOf(r)
	if v.Body != "" || len(v.Watchers) != 0 || v.Watchers == nil {
		t.Errorf("ShowViewOf of a broken task = %+v", v)
	}
}

func TestShowErrors(t *testing.T) {
	ts := newTestSession(t)
	_, err := ts.Show("ME-9")
	wantErr(t, err, KindNotFound, "task.not_found")
}

func TestLogAndTaskLog(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "a"}, {Title: "b"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.CommentAdd("ME-1", "hi"); err != nil {
		t.Fatal(err)
	}
	allLog, err := ts.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	all := entriesOf(allLog)
	ix := allLog.Logs[0].Index
	if ix == nil || allLog.Logs[0].Store != "me" {
		t.Fatalf("Log returned %+v", allLog.Logs[0])
	}
	twoLog, err := ts.Log(nil, 2)
	if err != nil || len(twoLog.Logs) != 2 || twoLog.Logs[0].Entry.Hash != all[0].Hash {
		t.Fatalf("Log(nil, 2) = %d commits, %v", len(twoLog.Logs), err)
	}
	oneLog, err := ts.Log([]string{"ME-1"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	one := entriesOf(oneLog)
	if oneLog.Logs[0].Store != "me" || oneLog.Logs[0].Index == nil {
		t.Errorf("Log(ME-1) = %+v", oneLog.Logs[0])
	}
	// ME-1: its add and its comment; not the add of ME-2.
	if len(one) != 2 {
		t.Fatalf("Log(ME-1) has %d commits; want 2", len(one))
	}
	e, _ := ix.Task(mustULID(t, mustTrailer(t, one[0], "Hedgehug-Task")))
	if e.ID != "ME-1" {
		t.Errorf("newest commit of ME-1 is about %s", e.ID)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	tl, err := ts.TaskLog(s, e.ULID, 1)
	if err != nil || len(tl) != 1 || tl[0].Hash != one[0].Hash {
		t.Errorf("TaskLog(limit 1) = %d commits, %v; want the newest", len(tl), err)
	}
	_, err = ts.Log([]string{"ME-9"}, 0)
	wantErr(t, err, KindNotFound, "task.not_found")
}

// entriesOf is the commits of lr.
func entriesOf(lr LogResult) []gitrepo.LogEntry {
	var out []gitrepo.LogEntry
	for _, l := range lr.Logs {
		out = append(out, l.Entry)
	}
	return out
}

func mustTrailer(t *testing.T, e gitrepo.LogEntry, key string) string {
	t.Helper()
	v, ok := e.Trailer(key)
	if !ok {
		t.Fatalf("no trailer %s", key)
	}
	return v
}

func mustULID(t *testing.T, s string) model.ULID {
	t.Helper()
	u, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestLogViewsOfAndSubjectNow(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	lr, err := ts.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ix, entries := lr.Logs[0].Index, entriesOf(lr)
	e, _ := ix.Task(mustULID(t, mustTrailer(t, entries[0], "Hedgehug-Task")))
	views := LogViewsOf(ix, entries)
	if len(views) != len(entries) {
		t.Fatalf("%d views for %d commits", len(views), len(entries))
	}
	v := views[0]
	if v.Hash != entries[0].Hash.String() || v.Subject != entries[0].Subject || v.Author.Name != "Alice" || v.Author.Email != "alice@example.com" {
		t.Errorf("view = %+v", v)
	}
	if v.Op == nil || *v.Op != "add" || v.Task == nil || *v.Task != e.ULID.String() || v.ID == nil || *v.ID != "ME-1" {
		t.Errorf("view trailers = op %v task %v id %v", v.Op, v.Task, v.ID)
	}
	if got, want := SubjectNow(ix, entries[0]), "hedgehug: add "+TaskLabel(e)+" Alpha"; got != want {
		t.Errorf("SubjectNow = %q; want %q", got, want)
	}

	// A subject written with an old label shows the label now.
	old := entries[0]
	old.Subject = "hedgehug: add OLD-7 Alpha"
	if got, want := SubjectNow(ix, old), "hedgehug: add "+TaskLabel(e)+" Alpha"; got != want {
		t.Errorf("SubjectNow(old label) = %q; want %q", got, want)
	}
	// No title: no trailing space.
	old.Subject = "hedgehug: add OLD-7"
	if got, want := SubjectNow(ix, old), "hedgehug: add "+TaskLabel(e); got != want {
		t.Errorf("SubjectNow(no title) = %q; want %q", got, want)
	}
	// Left as they are: no trailers, another op, a task the index lacks, a bad ULID.
	plain := gitrepo.LogEntry{Subject: "init"}
	if got := SubjectNow(ix, plain); got != "init" {
		t.Errorf("SubjectNow(plain) = %q", got)
	}
	other := entries[0]
	other.Subject = "hedgehug: edit X Alpha"
	if got := SubjectNow(ix, other); got != other.Subject {
		t.Errorf("SubjectNow(other op) = %q", got)
	}
	missing := entries[0]
	missing.Trailers = []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "add"}, {Key: "Hedgehug-Task", Value: model.NewULID().String()}}
	if got := SubjectNow(ix, missing); got != missing.Subject {
		t.Errorf("SubjectNow(missing task) = %q", got)
	}
	bad := entries[0]
	bad.Trailers = []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "add"}, {Key: "Hedgehug-Task", Value: "nope"}}
	if got := SubjectNow(ix, bad); got != bad.Subject {
		t.Errorf("SubjectNow(bad ULID) = %q", got)
	}
	// A commit without trailers has null op, task and id.
	pv := LogViewsOf(ix, []gitrepo.LogEntry{plain})[0]
	if pv.Op != nil || pv.Task != nil || pv.ID != nil {
		t.Errorf("view of a plain commit = %+v", pv)
	}
}
