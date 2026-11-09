package tasks_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestAddSetStatusDep(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a, err := tasks.Add(ts.Session, tasks.AddInput{Title: "parent"})
	if err != nil || !a.Committed {
		t.Fatalf("Add = %+v, %v", a, err)
	}
	b, err := tasks.Add(ts.Session, tasks.AddInput{Title: "child", Parent: "ME-1"})
	if err != nil {
		t.Fatal(err)
	}
	// The result holds an index it reads the new task through.
	if v, err := b.View(); err != nil || v.ID == "" || v.ParentRef == nil || v.ParentRef.ID != "ME-1" {
		t.Errorf("View of the child = %+v, %v; want it, under ME-1", v, err)
	}
	as, err := usecase.ParseAssigns([]string{"title=renamed", "labels+=x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-2", as); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.SetStatus(ts.Session, "ME-2", "doing"); usecase.KindOf(err) != usecase.KindInvalid {
		t.Errorf("SetStatus(unknown) kind = %v; want KindInvalid (%v)", usecase.KindOf(err), err)
	}
	if _, err := tasks.SetStatus(ts.Session, "ME-2", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"blocked_by": {"ME-2"}}, true); err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ts.Resolve(ix, "ME-2")
	if err != nil {
		t.Fatal(err)
	}
	if e.Title != "renamed" || e.Status != "done" {
		t.Errorf("ME-2 = %q %q; want renamed done", e.Title, e.Status)
	}
	p, _ := ts.Resolve(ix, "ME-1")
	if len(p.BlockedBy) != 1 || p.BlockedBy[0] != e.ULID {
		t.Errorf("ME-1 blocked_by = %v", p.BlockedBy)
	}
}

func TestAddBodyAndRepos(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	r, err := tasks.Add(ts.Session, tasks.AddInput{Title: "with a body", Body: "first\n\nsecond\n", Repos: []string{"github.com/o/r"}})
	if err != nil || !r.Committed {
		t.Fatalf("Add = %+v, %v", r, err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Task(r.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "first\n\nsecond\n" || !slices.Equal(got.Repos, []string{"github.com/o/r"}) {
		t.Errorf("body = %q, repos = %q", got.Body, got.Repos)
	}
}

func TestSetDeletedIsConflict(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Remove(ts.Session, "ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "x"}})
	usecasetest.WantErr(t, err, usecase.KindConflict, "task.deleted")
}

func TestParseAssignBadFormIsUsage(t *testing.T) {
	t.Parallel()
	_, err := usecase.ParseAssign("novalue")
	usecasetest.WantErr(t, err, usecase.KindUsage, "assign.form")
	_, err = usecase.ParseAssign("=x")
	usecasetest.WantErr(t, err, usecase.KindUsage, "assign.form")
}

func TestApplyAssignErrorKinds(t *testing.T) {
	t.Parallel()
	resolve := func(string) (model.ULID, error) { return model.ULID{}, nil }
	for _, tc := range []struct {
		in   string
		code string
	}{
		{"parent=x", "assign.readonly"},
		{"rank=x", "assign.readonly"},
		{"ulid=x", "assign.readonly"},
		{"nope=1", "assign.unknown_key"},
		{"title+=x", "assign.single"},
		{"due+=2026-11-01", "assign.single"},
		{"labels+=", "assign.missing_value"},
		{"tags-=", "assign.missing_value"},
		{"due=tomorrow", "assign.bad_value"},
		{"points=many", "assign.bad_value"},
		{"flag=maybe", "assign.bad_value"},
	} {
		a, err := usecase.ParseAssign(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		task := &model.Task{ULID: model.NewULID(), Kind: "task", Title: "t", Status: "todo"}
		usecasetest.WantErr(t, usecase.ApplyAssign(task, a, usecasetest.TestSchema(), resolve), usecase.KindInvalid, tc.code)
	}
}

func TestAddErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	ts.DeleteTask("ME-1")
	_, err := tasks.Add(ts.Session, tasks.AddInput{Title: "c", Parent: "ME-1"})
	usecasetest.WantErr(t, err, usecase.KindConflict, "task.parent_deleted")
	if !strings.Contains(err.Error(), "the parent ME-1") {
		t.Errorf("message = %q", err)
	}

	ts.AddWorkspace("other", "OT")
	_, err = tasks.Add(ts.Session, tasks.AddInput{Title: "d"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "workspace.choose")
	var e *usecase.Error
	if !usecasetest.AsError(err, &e) || !slices.Equal(e.Args["names"].([]string), []string{"me (ME)", "other (OT)"}) {
		t.Errorf("names = %+v", e)
	}

	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "p", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	_, err = tasks.Add(ts.Session, tasks.AddInput{Title: "q", Parent: "ME-2", Workspace: "other"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "task.workspace_mismatch")
	if !strings.Contains(err.Error(), "the parent is in workspace me") {
		t.Errorf("message = %q", err)
	}
	_, err = tasks.Add(ts.Session, tasks.AddInput{Title: "q", Workspace: "nope"})
	usecasetest.WantErr(t, err, usecase.KindNotFound, "")
}

func TestSetStatusUnknown(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := tasks.SetStatus(ts.Session, "ME-1", "nope")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "status.unknown")
	if !strings.Contains(err.Error(), `"nope" is not a status (statuses: `) {
		t.Errorf("message = %q", err)
	}
}

func TestDepErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, title := range []string{"a", "b"} {
		if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"blocked_by": {"ME-2"}}, false)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "dep.invalid")
	if _, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"blocked_by": {"ME-2"}}, true); err != nil {
		t.Fatal(err)
	}
	_, err = tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"blocked_by": {"ME-2"}}, true)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "dep.invalid")
	if r, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"blocked_by": {"ME-2"}}, false); err != nil || !r.Committed {
		t.Errorf("Dep remove = %+v, %v", r, err)
	}
}

// dep takes several relations and several tasks for each, and writes them in one commit.
func TestDepManyRelationsOneCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, title := range []string{"a", "b", "c", "d"} {
		ts.AddTask(title, nil)
	}
	head := ts.Head()
	r, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{
		"blocked_by": {"ME-2"}, "causes": {"ME-2", "ME-3"}, "relates": {"ME-3"}, "duplicates": {"ME-4"}, "clones": {"ME-4", "ME-2"},
	}, true)
	if err != nil || !r.Committed {
		t.Fatalf("Dep = %+v, %v", r, err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	es, err := usecasetest.LogOf(s.Repo(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if es[1].Hash.String() != head {
		t.Error("dep took more than one commit")
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ids := func(us []model.ULID) []string {
		var out []string
		for _, u := range us {
			e, _ := ix.Task(u)
			out = append(out, e.ID)
		}
		return out
	}
	a, _ := ts.Resolve(ix, "ME-1")
	for rel, want := range map[string][]string{
		"blocked_by": {"ME-2"}, "causes": {"ME-2", "ME-3"}, "relates": {"ME-3"}, "duplicates": {"ME-4"}, "clones": {"ME-4", "ME-2"},
	} {
		r, _, _ := schema.RelationOf(rel)
		if got := ids(r.Get(taskIn(t, ts, a.ULID))); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", rel, got, want)
		}
	}
	// Taking some off leaves the rest.
	if _, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"causes": {"ME-2"}, "clones": {"ME-4"}}, false); err != nil {
		t.Fatal(err)
	}
	got := taskIn(t, ts, a.ULID)
	if !slices.Equal(ids(got.Causes), []string{"ME-3"}) || !slices.Equal(ids(got.Clones), []string{"ME-2"}) {
		t.Errorf("causes = %v, clones = %v", ids(got.Causes), ids(got.Clones))
	}
}

// A pair that is already tied is refused for each target, nothing is written,
// and so are a task tied to itself and a name that is no relation.
func TestDepRefusals(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, title := range []string{"a", "b", "c"} {
		ts.AddTask(title, nil)
	}
	if _, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"relates": {"ME-2"}}, true); err != nil {
		t.Fatal(err)
	}
	head := ts.Head()
	_, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"relates": {"ME-2", "ME-3"}, "causes": {"ME-3"}}, true)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "dep.invalid")
	if ts.Head() != head {
		t.Error("a refused dep wrote")
	}
	_, err = tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"relates": {"ME-2"}, "causes": {"ME-3"}, "clones": {"ME-3"}}, false)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "dep.invalid")
	if msg := err.Error(); !strings.Contains(msg, "causes") || !strings.Contains(msg, "clones") || strings.Contains(msg, "relates") {
		t.Errorf("each pair on its own line; got %q", msg)
	}
	for _, rel := range []string{"relates", "causes", "duplicates", "clones"} {
		_, err := tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{rel: {"ME-1"}}, true)
		vs, ok := usecase.ViolationsOf(err)
		if !ok || len(vs) != 1 || vs[0].Code != schema.CodeSelfRef || vs[0].Field != rel || vs[0].Msg != "自分自身は指せません" {
			t.Errorf("%s to itself: err = %v", rel, err)
		}
	}
	_, err = tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"blocked_by": {"ME-1"}}, true)
	if vs, ok := usecase.ViolationsOf(err); !ok || len(vs) != 1 || vs[0].Code != schema.CodeCycle {
		t.Errorf("blocked_by to itself: err = %v, want a cycle", err)
	}
	_, err = tasks.Dep(ts.Session, "ME-1", tasks.DepTargets{"blocks": {"ME-2"}}, true)
	usecasetest.WantErr(t, err, usecase.KindUsage, "dep.unknown_relation")
}

func TestWatchAddsAndRemovesMe(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	// The writer of the task watches it; take them off, then drop the account.
	if _, err := tasks.Watch(ts.Session, "ME-1", false); err != nil {
		t.Fatal(err)
	}
	ts.DropDefaultAccount()
	// With no account in config.yaml there is no "me" to watch with.
	_, err := tasks.Watch(ts.Session, "ME-1", true)
	if err == nil || !strings.Contains(err.Error(), "no account is set up") {
		t.Fatalf("Watch without an account: %v", err)
	}
	var ne *usecase.Error
	if usecase.KindOf(err) != usecase.KindConflict || !errors.As(err, &ne) || ne.Code != "account.none" {
		t.Fatalf("Watch without an account: kind %v, err %v; want KindConflict account.none", usecase.KindOf(err), err)
	}

	ts.UseAccount() // a new session: the writer is read again
	watchers := func() []string {
		t.Helper()
		_, ix, err := ts.Open()
		if err != nil {
			t.Fatal(err)
		}
		e, err := ts.Resolve(ix, "ME-1")
		if err != nil {
			t.Fatal(err)
		}
		return e.Watchers
	}
	r, err := tasks.Watch(ts.Session, "ME-1", true)
	if err != nil || !r.Committed {
		t.Fatalf("Watch = %+v, %v", r, err)
	}
	if got := watchers(); !slices.Equal(got, []string{"github.com/alice"}) {
		t.Errorf("watchers = %v", got)
	}
	if r, err := tasks.Watch(ts.Session, "ME-1", true); err != nil || r.Committed {
		t.Errorf("Watch again = %+v, %v; want no commit", r, err)
	}
	if r, err := tasks.Watch(ts.Session, "ME-1", false); err != nil || !r.Committed {
		t.Fatalf("Unwatch = %+v, %v", r, err)
	}
	if got := watchers(); len(got) != 0 {
		t.Errorf("watchers after unwatch = %v", got)
	}
}

func TestSiblingsAndEndRank(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, title := range []string{"a", "b", "c"} {
		if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "kid", Parent: "ME-1"}); err != nil {
		t.Fatal(err)
	}
	ts.DeleteTask("ME-3")
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := ix.Workspaces()[0].ULID
	titles := func(es []*index.Entry) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.Title)
		}
		return out
	}
	if got := titles(usecase.Siblings(ix, nil, ws, model.ULID{})); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("root siblings = %v; want [a b] (kid is a child, c is deleted)", got)
	}
	b, _ := ts.Resolve(ix, "ME-2")
	if got := titles(usecase.Siblings(ix, nil, ws, b.ULID)); !slices.Equal(got, []string{"a"}) {
		t.Errorf("siblings without b = %v", got)
	}
	a, _ := ts.Resolve(ix, "ME-1")
	if got := titles(usecase.Siblings(ix, &a.ULID, ws, model.ULID{})); !slices.Equal(got, []string{"kid"}) {
		t.Errorf("children of a = %v", got)
	}
	if got := usecase.Siblings(ix, nil, model.NewULID(), model.ULID{}); len(got) != 0 {
		t.Errorf("siblings in another workspace = %v", got)
	}
	r, err := usecase.EndRank(ix, nil, ws, model.ULID{})
	if err != nil {
		t.Fatal(err)
	}
	if r <= b.Rank {
		t.Errorf("EndRank = %q; want after %q", r, b.Rank)
	}
	if r, err := usecase.EndRank(ix, &b.ULID, ws, model.ULID{}); err != nil || r == "" {
		t.Errorf("EndRank under a task without children = %q, %v", r, err)
	}
}

func TestNotDeletedAndLiveParent(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	e, _ := ts.Resolve(ix, "ME-1")
	if err := usecase.NotDeleted(ix, e); err != nil {
		t.Errorf("NotDeleted(live) = %v", err)
	}
	if err := usecase.LiveParent(ix, e); err != nil {
		t.Errorf("LiveParent(live) = %v", err)
	}
	ts.DeleteTask("ME-1")
	_, ix, _ = ts.Open()
	e, _ = ts.Resolve(ix, "ME-1")
	err := usecase.NotDeleted(ix, e)
	usecasetest.WantErr(t, err, usecase.KindConflict, "task.deleted")
	if !strings.Contains(err.Error(), "ME-1") || !strings.Contains(err.Error(), "is deleted; restore ME-1 first") {
		t.Errorf("message = %q", err)
	}
	err = usecase.LiveParent(ix, e)
	usecasetest.WantErr(t, err, usecase.KindConflict, "task.parent_deleted")
	if !strings.HasPrefix(err.Error(), "the parent ME-1") {
		t.Errorf("message = %q", err)
	}
}

func TestResolveWorkspace(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("zz", "OT")
	ts.AddWorkspace("ot", "XX") // its name is the prefix of the one before
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if w, err := usecase.ResolveWorkspace(ix, "xx"); err != nil || w.Name != "ot" {
		t.Errorf("ResolveWorkspace(zz) = %v, %v", w, err)
	}
	_, err = usecase.ResolveWorkspace(ix, "ot")
	usecasetest.WantErr(t, err, usecase.KindAmbiguous, "workspace.ambiguous")
	if !strings.Contains(err.Error(), `"ot" matches 2 workspaces:`) {
		t.Errorf("message = %q", err)
	}
	_, err = usecase.ResolveWorkspace(ix, "nope")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "")
}

func TestWatchInTx(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	task := ts.AddTask("a", nil)
	s, _, _ := ts.Open()
	run := func(accts ...string) bool {
		t.Helper()
		committed, err := ts.Update(s, store.Op{Name: "comment", Task: task.ULID}, func(tx *store.Tx) error {
			return usecase.WatchInTx(tx, task.ULID, accts...)
		})
		if err != nil {
			t.Fatal(err)
		}
		return committed
	}
	if run() {
		t.Error("no accounts committed")
	}
	if !run("github.com/bob") {
		t.Error("a new watcher did not commit")
	}
	if run("GitHub.com/Bob") {
		t.Error("a watcher that differs in case committed")
	}
	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Watchers, []string{"github.com/bob"}) {
		t.Errorf("watchers = %v", got.Watchers)
	}
}

func TestNewMentions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		old, now, host string
		want           []string
	}{
		{"", "ping @carol", "github.com", []string{"github.com/carol"}},
		{"", "see @example.com/dave", "github.com", []string{"example.com/dave"}},
		{"hi @carol", "hi @carol and @bob", "github.com", []string{"github.com/bob"}},
		{"hi @Carol", "hi @carol", "github.com", nil},
		{"", "ping @carol", "", nil}, // no writer host: a bare login names no account
		{"", "ping @example.com/dave", "", []string{"example.com/dave"}},
	} {
		if got := usecase.NewMentions(tc.old, tc.now, tc.host); !slices.Equal(got, tc.want) {
			t.Errorf("NewMentions(%q, %q, %q) = %v; want %v", tc.old, tc.now, tc.host, got, tc.want)
		}
	}
}

func TestWriterHost(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.DropDefaultAccount()
	if h, err := ts.WriterHost(); err != nil || h != "" {
		t.Errorf("WriterHost without an account = %q, %v", h, err)
	}
	ts.UseAccount()
	if h, err := ts.WriterHost(); err != nil || h != "github.com" {
		t.Errorf("WriterHost = %q, %v; want github.com", h, err)
	}
}
