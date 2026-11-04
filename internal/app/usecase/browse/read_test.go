package browse_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/query"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

func mustParse(t *testing.T, s string) *query.Query {
	t.Helper()
	q, err := usecase.ParseQuery(s)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func ids(vs []*usecase.TaskView) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.ID)
	}
	return out
}

// commitsOf is the commits of the store of ts, newest first, and its index.
func commitsOf(t *testing.T, ts *usecasetest.Session) (*index.Index, []gitrepo.LogEntry) {
	t.Helper()
	s, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	h, err := ts.OpenHistory(s, "me")
	if err != nil {
		t.Fatal(err)
	}
	es, err := h.Entries(h.All(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return ix, es
}

func TestListTreeShowLog(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, in := range []tasks.AddInput{{Title: "p"}, {Title: "c", Parent: "ME-1"}, {Title: "other"}} {
		if _, err := tasks.Add(ts.Session, in); err != nil {
			t.Fatal(err)
		}
	}
	l, err := browse.List(ts.Session, "c")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Tasks) != 1 || l.Tasks[0].ID != "ME-2" || l.Tasks[0].Store != "me" || l.Many {
		t.Fatalf("List = %+v", l)
	}
	tr, err := browse.Tree(ts.Session, "c")
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Stores) != 1 || tr.Many {
		t.Fatalf("Tree has %d stores, many %v; want one", len(tr.Stores), tr.Many)
	}
	roots := tr.Stores[0].Roots
	if len(roots) != 1 || roots[0].ID != "ME-1" || !roots[0].Context {
		t.Fatalf("Tree roots = %+v; want ME-1 as context", roots)
	}
	if c := roots[0].Children; len(c) != 1 || c[0].ID != "ME-2" || c[0].Context {
		t.Fatalf("Tree children = %+v", c)
	}
	sh, err := browse.Show(ts.Session, "ME-2")
	if err != nil {
		t.Fatal(err)
	}
	if sh.Title != "c" || len(sh.History) != 1 {
		t.Errorf("Show = %q with %d commits; want c with 1", sh.Title, len(sh.History))
	}
	lr, err := browse.Log(ts.Session, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.Logs) < 3 || lr.Many {
		t.Errorf("Log has %d commits, many %v; want at least 3 of one store", len(lr.Logs), lr.Many)
	}
}

func TestParseQuery(t *testing.T) {
	t.Parallel()
	q, err := usecase.ParseQuery("status:todo group:status")
	if err != nil || len(q.Terms) != 2 {
		t.Fatalf("ParseQuery = %+v, %v; want two terms", q, err)
	}
	_, err = usecase.ParseQuery(`title:"unclosed`)
	usecasetest.WantErr(t, err, usecase.KindUsage, "query.invalid")
}

func TestListGroupsAndErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, in := range []tasks.AddInput{{Title: "a"}, {Title: "b", Status: "done"}} {
		if _, err := tasks.Add(ts.Session, in); err != nil {
			t.Fatal(err)
		}
	}
	l, err := browse.List(ts.Session, "group:status sort:title")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Groups) != 2 || len(l.Tasks) != 2 {
		t.Fatalf("List = %d groups, %d tasks", len(l.Groups), len(l.Tasks))
	}
	if l.Tasks[0].Title != l.Groups[0].Tasks[0].Title || l.Tasks[1].Title != l.Groups[1].Tasks[0].Title {
		t.Errorf("Tasks are not the groups' tasks in order: %v", l.Tasks)
	}
	if l.Groups[0].ULIDs[0] != l.Groups[0].Tasks[0].ULID {
		t.Errorf("group ULIDs = %v; want those of its tasks", l.Groups[0].ULIDs)
	}
	if j := l.JSON(); j.Tasks == nil || len(*j.Tasks) != 2 || len(j.Groups) != 2 || j.Roots != nil {
		t.Errorf("List JSON = %+v", j)
	}
	// A key the schema does not have parses, and fails when it runs.
	_, err = browse.List(ts.Session, "nope:x")
	usecasetest.WantErr(t, err, usecase.KindUsage, "query.invalid")
}

func TestHasTerm(t *testing.T) {
	t.Parallel()
	q := mustParse(t, "status:todo sort:title")
	if !usecase.HasTerm(q, "sort") || !usecase.HasTerm(q, "status") || usecase.HasTerm(q, "group") {
		t.Errorf("HasTerm wrong for %+v", q.Terms)
	}
	if usecase.HasTerm(&query.Query{}, "sort") {
		t.Error("HasTerm on an empty query = true")
	}
}

func TestTreeRefusesGroupAndSort(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, s := range []string{"group:status", "sort:title"} {
		_, err := browse.Tree(ts.Session, s)
		usecasetest.WantErr(t, err, usecase.KindUsage, "query.tree_order")
		if want := "--tree orders tasks by rank; it takes neither group: nor sort:"; err != nil && err.Error() != want {
			t.Errorf("%s: message = %q; want %q", s, err, want)
		}
	}
}

func TestTreeViewsAndDrawn(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, in := range []tasks.AddInput{{Title: "p"}, {Title: "c1", Parent: "ME-1"}, {Title: "g", Parent: "ME-2"}, {Title: "c2", Parent: "ME-1"}, {Title: "q"}} {
		if _, err := tasks.Add(ts.Session, in); err != nil {
			t.Fatal(err)
		}
	}
	tr, err := browse.Tree(ts.Session, "g")
	if err != nil {
		t.Fatal(err)
	}
	// Only the ancestors of g are drawn, c2 and q are not.
	if got := ids(tr.Stores[0].Drawn()); !slices.Equal(got, []string{"ME-1", "ME-2", "ME-3"}) {
		t.Fatalf("Drawn = %v; want ME-1 ME-2 ME-3", got)
	}
	if len(tr.Stores[0].Cycle) != 0 {
		t.Errorf("Cycle = %v; want none", ids(tr.Stores[0].Cycle))
	}
	v := tr.Stores[0].Roots[0]
	if v.ID != "ME-1" || !v.Context || v.InCycle || len(v.Children) != 1 {
		t.Fatalf("tree root = %+v", v)
	}
	mid := v.Children[0]
	if mid.ID != "ME-2" || !mid.Context || len(mid.Children) != 1 {
		t.Fatalf("tree middle = %+v", mid)
	}
	if leaf := mid.Children[0]; leaf.ID != "ME-3" || leaf.Context || len(leaf.Children) != 0 {
		t.Errorf("tree leaf = %+v", leaf)
	}
	// The whole forest: two roots, in rank order, depth first.
	all, err := browse.Tree(ts.Session, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(all.Stores[0].Drawn()); !slices.Equal(got, []string{"ME-1", "ME-2", "ME-3", "ME-4", "ME-5"}) {
		t.Errorf("Drawn(all) = %v", got)
	}
	if (browse.TreeStoreView{}).Drawn() != nil {
		t.Error("Drawn of an empty store is not nil")
	}
	if j := all.JSON(); j.Roots == nil || len(*j.Roots) != 2 || j.Cycle != nil || j.Tasks != nil {
		t.Errorf("Tree JSON = %+v; want two roots and no cycle", j)
	}
}

// rawTask commits a task file past the store's checks, as a sync or a hand
// edit could.
func rawTask(t *testing.T, ts *usecasetest.Session, id string, ulid model.ULID, front string) {
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
	author := config.CommitAuthor(usecasetest.TestAccount)
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func TestTreeParentCycle(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	x, y, c := model.NewULID(), model.NewULID(), model.NewULID()
	rawTask(t, ts, "ME-1", x, "title: X\nparent: "+y.String()+"\n")
	rawTask(t, ts, "ME-2", y, "title: Y\nparent: "+x.String()+"\n")
	rawTask(t, ts, "ME-3", c, "title: C\nparent: "+x.String()+"\n")
	tr, err := browse.Tree(ts.Session, "")
	if err != nil {
		t.Fatal(err)
	}
	ts0 := tr.Stores[0]
	if len(ts0.Roots) != 0 || len(ts0.Cycle) != 2 {
		t.Fatalf("Roots = %v, Cycle = %v; want no roots and the two members", ids(ts0.Roots), ids(ts0.Cycle))
	}
	for _, n := range ts0.Cycle {
		if !n.InCycle {
			t.Errorf("%s is not marked InCycle", n.ID)
		}
	}
	// C hangs under exactly one member and is not itself in the cycle.
	var under *usecase.TaskView
	for _, n := range ts0.Cycle {
		for _, ch := range n.Children {
			if ch.ID == "ME-3" {
				under = ch
			}
		}
	}
	if under == nil || under.InCycle {
		t.Fatalf("C is not drawn under a cycle member, or is marked InCycle: %+v", under)
	}
	if j := tr.JSON(); j.Roots == nil || len(*j.Roots) != 0 || len(j.Cycle) != 2 {
		t.Errorf("Tree JSON = %+v; want no roots and the two members of the cycle", j)
	}
}

func TestShowFullTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	a := ts.AddTask("a", func(t *model.Task) { t.Body = "the body\n"; t.Watchers = []string{"bob"}; t.Repos = []string{"r1"} })
	b := ts.AddTask("b", nil)
	c := ts.AddTask("c", nil)
	// b is blocked by a; c duplicates a; c writes a as #ME-1 in its body.
	if _, err := tasks.Dep(ts.Session, "ME-2", "ME-1", true); err != nil {
		t.Fatal(err)
	}
	as, err := usecase.ParseAssigns([]string{"duplicates+=ME-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-3", as); err != nil {
		t.Fatal(err)
	}
	_ = ts.AddTask("d", func(t *model.Task) { t.Body = "see #ME-1\n" })
	if _, err := comments.CommentAdd(ts.Session, "ME-1", "first"); err != nil {
		t.Fatal(err)
	}
	gone, err := comments.CommentAdd(ts.Session, "ME-1", "second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := comments.CommentAdd(ts.Session, "ME-1", "third"); err != nil {
		t.Fatal(err)
	}
	if _, err := comments.CommentRemove(ts.Session, gone.Comment.ULID.String()); err != nil {
		t.Fatal(err)
	}

	r, err := browse.Show(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if r.ULID != a.ULID.String() || r.Body != "the body\n" {
		t.Fatalf("Show = %v with body %q", r.ID, r.Body)
	}
	var cs []string
	for _, cm := range r.Comments {
		cs = append(cs, cm.Body)
	}
	if !slices.Equal(cs, []string{"first\n", "third\n"}) {
		t.Errorf("Comments = %q; want first and third (the removed one is left out)", cs)
	}
	if len(r.Relations) != 2 || r.Relations[0].Key != "blocks" || r.Relations[0].Label != "blocks" || r.Relations[0].Tasks[0].ULID != b.ULID.String() {
		t.Fatalf("Relations = %+v", r.Relations)
	}
	if rel := r.Relations[1]; rel.Key != "duplicates" || rel.Label != "重複されている" || len(rel.Tasks) != 1 || rel.Tasks[0].ULID != c.ULID.String() {
		t.Errorf("Relations[1] = %+v; want duplicates / 重複されている / c", rel)
	}
	if len(r.ReferencedBy) != 1 || r.ReferencedBy[0].Title != "d" || r.ReferencedBy[0].Name() == "" {
		t.Errorf("ReferencedBy = %v; want d", r.ReferencedBy)
	}
	if len(r.History) == 0 || len(r.History) > browse.ShowHistory {
		t.Errorf("History has %d commits; want 1 to %d", len(r.History), browse.ShowHistory)
	}

	v := r
	if v.ID != "ME-1" || v.Body != "the body\n" || !slices.Equal(v.Watchers, []string{"bob", "github.com/alice"}) || !slices.Equal(v.Repos, []string{"r1"}) {
		t.Errorf("Show = %+v", v)
	}
	if len(v.Comments) != 2 || v.Comments[0].Body != "first\n" || v.Comments[0].ULID == "" {
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
	if len(v.History) == 0 || v.History[0].ID == nil || *v.History[0].ID != "ME-1" {
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

func TestShowEmpty(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "bare"}); err != nil {
		t.Fatal(err)
	}
	// The writer of the add is a watcher; take it off for the empty view.
	if _, err := tasks.Watch(ts.Session, "ME-1", false); err != nil {
		t.Fatal(err)
	}
	r, err := browse.Show(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Comments) != 0 || len(r.Relations) != 0 || len(r.ReferencedBy) != 0 {
		t.Errorf("Show = %d comments, %d relations, %d back; want none", len(r.Comments), len(r.Relations), len(r.ReferencedBy))
	}
	v := r
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
	t.Parallel()
	ts := usecasetest.New(t)
	ulid := model.NewULID()
	rawTask(t, ts, "ME-1", ulid, "title: [unclosed\n")
	r, err := browse.Show(ts.Session, ulid.String())
	if err != nil {
		t.Fatal(err)
	}
	if r.Broken == "" || len(r.Comments) != 0 {
		t.Fatalf("Show = broken %q, comments %v; want a broken task with no comments", r.Broken, r.Comments)
	}
	if r.Body != "" || len(r.Watchers) != 0 || r.Watchers == nil || r.CreatedBy != "" {
		t.Errorf("Show of a broken task = %+v", r)
	}
}

func TestShowErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := browse.Show(ts.Session, "ME-9")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "task.not_found")
}

func TestLogAndTaskLog(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, in := range []tasks.AddInput{{Title: "a"}, {Title: "b"}} {
		if _, err := tasks.Add(ts.Session, in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := comments.CommentAdd(ts.Session, "ME-1", "hi"); err != nil {
		t.Fatal(err)
	}
	allLog, err := browse.Log(ts.Session, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ix, all := commitsOf(t, ts)
	if len(allLog.Logs) != len(all) || allLog.Logs[0].Store != "me" || allLog.Logs[0].Hash != all[0].Hash.String() {
		t.Fatalf("Log returned %d commits, the first %+v", len(allLog.Logs), allLog.Logs[0])
	}
	twoLog, err := browse.Log(ts.Session, nil, 2)
	if err != nil || len(twoLog.Logs) != 2 || twoLog.Logs[0].Hash != all[0].Hash.String() {
		t.Fatalf("Log(nil, 2) = %d commits, %v", len(twoLog.Logs), err)
	}
	oneLog, err := browse.Log(ts.Session, []string{"ME-1"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	one := oneLog.Logs
	if one[0].Store != "me" {
		t.Errorf("Log(ME-1) = %+v", one[0])
	}
	// ME-1: its add and its comment; not the add of ME-2.
	if len(one) != 2 {
		t.Fatalf("Log(ME-1) has %d commits; want 2", len(one))
	}
	taskOf, _ := one[0].Trailer("Hedgehug-Task")
	e, _ := ix.Task(usecasetest.MustULID(t, taskOf))
	if e.ID != "ME-1" {
		t.Errorf("newest commit of ME-1 is about %s", e.ID)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	tl, err := browse.TaskLog(ts.Session, s, "me", e.ULID, 1)
	if err != nil || len(tl) != 1 || tl[0].Hash.String() != one[0].Hash {
		t.Errorf("TaskLog(limit 1) = %d commits, %v; want the newest", len(tl), err)
	}
	_, err = browse.Log(ts.Session, []string{"ME-9"}, 0)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "task.not_found")
}

func TestTaskLogReportsProgressOfBuildingTheIndex(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	type call struct {
		store       string
		done, total int
	}
	var calls []call
	ts.Progress = func(store string, done, total int) { calls = append(calls, call{store, done, total}) }
	if _, err := browse.Log(ts.Session, []string{"ME-1"}, 0); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 {
		t.Fatal("Progress was not called for a store without an index")
	}
	if last := calls[len(calls)-1]; last.store != "me" || last.done != last.total || last.total < 2 {
		t.Errorf("last Progress call = %+v, want store me, done == total >= 2 (init and add)", last)
	}
	// the index is in place now: nothing is left to do
	calls = nil
	if _, err := browse.Log(ts.Session, []string{"ME-1"}, 0); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Errorf("Progress called %d times with the index in place", len(calls))
	}
}

func mustTrailer(t *testing.T, e gitrepo.LogEntry, key string) string {
	t.Helper()
	v, ok := e.Trailer(key)
	if !ok {
		t.Fatalf("no trailer %s", key)
	}
	return v
}

func TestLogViewOfAndCurrentSubject(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	ix, entries := commitsOf(t, ts)
	e, _ := ix.Task(usecasetest.MustULID(t, mustTrailer(t, entries[0], "Hedgehug-Task")))
	v := usecase.LogViewOf(ix, entries[0])
	if v.Hash != entries[0].Hash.String() || v.Subject != entries[0].Subject || v.Author.Name != "alice" || v.Author.Email != "alice@users.noreply.github.com" {
		t.Errorf("view = %+v", v)
	}
	if v.Op == nil || *v.Op != "add" || v.Task == nil || *v.Task != e.ULID.String() || v.ID == nil || *v.ID != "ME-1" {
		t.Errorf("view trailers = op %v task %v id %v", v.Op, v.Task, v.ID)
	}
	if got, want := usecase.LogViewOf(ix, entries[0]).CurrentSubject, "hedgehug: add "+usecase.TaskLabel(ix, e)+" Alpha"; got != want {
		t.Errorf("CurrentSubject = %q; want %q", got, want)
	}

	// A subject written with an old label shows the label now.
	old := entries[0]
	old.Subject = "hedgehug: add OLD-7 Alpha"
	if got, want := usecase.LogViewOf(ix, old).CurrentSubject, "hedgehug: add "+usecase.TaskLabel(ix, e)+" Alpha"; got != want {
		t.Errorf("CurrentSubject(old label) = %q; want %q", got, want)
	}
	// No title: no trailing space.
	old.Subject = "hedgehug: add OLD-7"
	if got, want := usecase.LogViewOf(ix, old).CurrentSubject, "hedgehug: add "+usecase.TaskLabel(ix, e); got != want {
		t.Errorf("CurrentSubject(no title) = %q; want %q", got, want)
	}
	// Left as they are: no trailers, another op, a task the index lacks, a bad ULID.
	plain := gitrepo.LogEntry{Subject: "init"}
	if got := usecase.LogViewOf(ix, plain).CurrentSubject; got != "init" {
		t.Errorf("CurrentSubject(plain) = %q", got)
	}
	other := entries[0]
	other.Subject = "hedgehug: edit X Alpha"
	if got := usecase.LogViewOf(ix, other).CurrentSubject; got != other.Subject {
		t.Errorf("CurrentSubject(other op) = %q", got)
	}
	missing := entries[0]
	missing.Trailers = []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "add"}, {Key: "Hedgehug-Task", Value: model.NewULID().String()}}
	if got := usecase.LogViewOf(ix, missing).CurrentSubject; got != missing.Subject {
		t.Errorf("CurrentSubject(missing task) = %q", got)
	}
	bad := entries[0]
	bad.Trailers = []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "add"}, {Key: "Hedgehug-Task", Value: "nope"}}
	if got := usecase.LogViewOf(ix, bad).CurrentSubject; got != bad.Subject {
		t.Errorf("CurrentSubject(bad ULID) = %q", got)
	}
	// A commit without trailers has null op, task and id.
	pv := usecase.LogViewOf(ix, plain)
	if pv.Op != nil || pv.Task != nil || pv.ID != nil {
		t.Errorf("view of a plain commit = %+v", pv)
	}
}
