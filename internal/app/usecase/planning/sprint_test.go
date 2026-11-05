package planning_test

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/planning"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func addSprint(t *testing.T, ts *usecasetest.Session, name string) planning.SprintWrite {
	t.Helper()
	w, err := planning.SprintAdd(ts.Session, planning.SprintAddInput{Name: name, Start: "2026-11-02", End: "2026-11-13"})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func taskOf(t *testing.T, ts *usecasetest.Session, u model.ULID) *model.Task {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Task(u)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func inSprint(t *testing.T, w planning.SprintWrite) func(*model.Task) {
	t.Helper()
	u, err := model.ParseULID(w.Sprint.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return func(task *model.Task) { task.Sprint = &u }
}

func commitCount(t *testing.T, ts *usecasetest.Session) int {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	es, err := usecasetest.LogOf(s.Repo(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return len(es)
}

func TestSprintAdd(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	w := addSprint(t, ts, "Sprint 1")
	if w.Sprint.State != model.SprintPlanned || w.Sprint.Start != "2026-11-02" || w.Commit == nil {
		t.Fatalf("write = %+v", w)
	}
	if got := ts.HeadSubject(); !strings.Contains(got, "sprint-add") || !strings.Contains(got, "Sprint 1") {
		t.Errorf("subject = %q", got)
	}

	for _, tc := range []struct {
		in   planning.SprintAddInput
		kind usecase.Kind
		code string
	}{
		{planning.SprintAddInput{Name: "sprint 1"}, usecase.KindConflict, "sprint.name_taken"},
		{planning.SprintAddInput{Name: "next"}, usecase.KindUsage, "sprint.bad_name"},
		{planning.SprintAddInput{Name: "Backlog"}, usecase.KindUsage, "sprint.bad_name"},
		{planning.SprintAddInput{Name: "a,b"}, usecase.KindUsage, "sprint.bad_name"},
		{planning.SprintAddInput{Name: " a"}, usecase.KindUsage, "sprint.bad_name"},
		{planning.SprintAddInput{Name: ""}, usecase.KindUsage, "sprint.bad_name"},
		{planning.SprintAddInput{Name: "s", Start: "soon"}, usecase.KindUsage, "assign.bad_value"},
		{planning.SprintAddInput{Name: "s", Start: "2026-11-13", End: "2026-11-02"}, usecase.KindInvalid, "sprint.bad_dates"},
	} {
		_, err := planning.SprintAdd(ts.Session, tc.in)
		usecasetest.WantErr(t, err, tc.kind, tc.code)
	}
}

func TestSprintStart(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := planning.SprintAdd(ts.Session, planning.SprintAddInput{Name: "undated"}); err != nil {
		t.Fatal(err)
	}
	_, err := planning.SprintStart(ts.Session, "undated", "")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "sprint.no_dates")

	addSprint(t, ts, "s1")
	w, err := planning.SprintStart(ts.Session, "S1", "")
	if err != nil {
		t.Fatal(err)
	}
	if w.Sprint.State != model.SprintActive {
		t.Fatalf("state = %s", w.Sprint.State)
	}
	_, err = planning.SprintStart(ts.Session, "s1", "")
	usecasetest.WantErr(t, err, usecase.KindConflict, "sprint.not_planned")
	_, err = planning.SprintStart(ts.Session, "nope", "")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "sprint.not_found")
}

// closeFixture has the active sprint s1 with an open, a doing, a done and a
// canceled task, and the planned sprint s2.
type closeFixture struct {
	ts                     *usecasetest.Session
	open, doing, done, cut model.ULID
	s2                     model.ULID
}

func newCloseFixture(t *testing.T) *closeFixture {
	t.Helper()
	ts := usecasetest.New(t)
	s1 := addSprint(t, ts, "s1")
	s2 := addSprint(t, ts, "s2")
	add := func(title, status string) model.ULID {
		return ts.AddTask(title, func(task *model.Task) {
			inSprint(t, s1)(task)
			task.Status = status
		}).ULID
	}
	f := &closeFixture{ts: ts, open: add("o", "todo"), doing: add("d", "in_progress"), done: add("x", "done"), cut: add("c", "canceled")}
	var err error
	if f.s2, err = model.ParseULID(s2.Sprint.ULID); err != nil {
		t.Fatal(err)
	}
	if _, err := planning.SprintStart(ts.Session, "s1", ""); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSprintCloseToNext(t *testing.T) {
	t.Parallel()
	f := newCloseFixture(t)
	before := commitCount(t, f.ts)
	n, err := planning.SprintOpenTasks(f.ts.Session, "s1", "")
	if err != nil || n != 2 {
		t.Fatalf("SprintOpenTasks = %d, %v", n, err)
	}
	w, err := planning.SprintClose(f.ts.Session, "s1", "", planning.ToNext)
	if err != nil {
		t.Fatal(err)
	}
	if w.Sprint.State != model.SprintClosed || w.Moved != 2 || w.To != "s2" {
		t.Fatalf("write = %+v", w)
	}
	if got := commitCount(t, f.ts); got != before+1 {
		t.Errorf("%d commits for one close", got-before)
	}
	for _, u := range []model.ULID{f.open, f.doing} {
		if s := taskOf(t, f.ts, u).Sprint; s == nil || *s != f.s2 {
			t.Errorf("open task is in %v, want s2", s)
		}
	}
	s1 := mustSprintULID(t, f.ts, "s1")
	for _, u := range []model.ULID{f.done, f.cut} {
		if s := taskOf(t, f.ts, u).Sprint; s == nil || *s != s1 {
			t.Errorf("finished task moved to %v", s)
		}
	}
}

func mustSprintULID(t *testing.T, ts *usecasetest.Session, name string) model.ULID {
	t.Helper()
	vs, err := planning.SprintList(ts.Session, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.Name == name {
			u, err := model.ParseULID(v.ULID)
			if err != nil {
				t.Fatal(err)
			}
			return u
		}
	}
	t.Fatalf("no sprint %s", name)
	return model.ULID{}
}

func TestSprintCloseToBacklogAndNamed(t *testing.T) {
	t.Parallel()
	f := newCloseFixture(t)
	if _, err := planning.SprintClose(f.ts.Session, "s1", "", "backlog"); err != nil {
		t.Fatal(err)
	}
	if s := taskOf(t, f.ts, f.open).Sprint; s != nil {
		t.Errorf("sprint = %v, want none", s)
	}

	g := newCloseFixture(t)
	w, err := planning.SprintClose(g.ts.Session, "s1", "", "S2")
	if err != nil {
		t.Fatal(err)
	}
	if w.To != "s2" {
		t.Errorf("to = %q", w.To)
	}
}

func TestSprintCloseRefusals(t *testing.T) {
	t.Parallel()
	f := newCloseFixture(t)
	head := f.ts.Head()
	_, err := planning.SprintClose(f.ts.Session, "s1", "", "")
	usecasetest.WantErr(t, err, usecase.KindUsage, "sprint.close_to")
	_, err = planning.SprintClose(f.ts.Session, "s1", "", "s1")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "sprint.bad_target")
	_, err = planning.SprintClose(f.ts.Session, "s1", "", "nope")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "sprint.not_found")
	_, err = planning.SprintClose(f.ts.Session, "s2", "", "backlog")
	usecasetest.WantErr(t, err, usecase.KindConflict, "sprint.not_active")
	if f.ts.Head() != head {
		t.Error("a refused close made a commit")
	}

	// With no planned sprint, next is refused and nothing changes.
	if _, err := planning.SprintRemove(f.ts.Session, "s2", ""); err != nil {
		t.Fatal(err)
	}
	head = f.ts.Head()
	_, err = planning.SprintClose(f.ts.Session, "s1", "", planning.ToNext)
	usecasetest.WantErr(t, err, usecase.KindConflict, "sprint.no_next")
	if f.ts.Head() != head {
		t.Error("a refused close made a commit")
	}
}

func TestSprintCloseWithNothingOpen(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	addSprint(t, ts, "s1")
	if _, err := planning.SprintStart(ts.Session, "s1", ""); err != nil {
		t.Fatal(err)
	}
	w, err := planning.SprintClose(ts.Session, "s1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if w.Moved != 0 || w.To != "" || w.Sprint.State != model.SprintClosed {
		t.Fatalf("write = %+v", w)
	}
}

func TestSprintSet(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	addSprint(t, ts, "s1")
	addSprint(t, ts, "s2")
	assigns := func(ss ...string) []usecase.Assign {
		as, err := usecase.ParseAssigns(ss)
		if err != nil {
			t.Fatal(err)
		}
		return as
	}
	w, err := planning.SprintSet(ts.Session, "s1", "", assigns("name=first", "goal=ship it", "end=2026-11-20"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Sprint.Name != "first" || w.Sprint.Goal != "ship it" || w.Sprint.End != "2026-11-20" {
		t.Fatalf("write = %+v", w)
	}
	for _, tc := range []struct {
		set  string
		kind usecase.Kind
		code string
	}{
		{"state=active", usecase.KindInvalid, "assign.readonly"},
		{"name=s2", usecase.KindConflict, "sprint.name_taken"},
		{"color=red", usecase.KindInvalid, "assign.unknown_key"},
		{"end=2026-01-01", usecase.KindInvalid, "sprint.bad_dates"},
	} {
		_, err := planning.SprintSet(ts.Session, "first", "", assigns(tc.set))
		usecasetest.WantErr(t, err, tc.kind, tc.code)
	}
}

func TestSprintRemoveTakesItOffTasks(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	s1 := addSprint(t, ts, "s1")
	a := ts.AddTask("a", inSprint(t, s1))
	before := commitCount(t, ts)
	if _, err := planning.SprintRemove(ts.Session, "s1", ""); err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, ts); got != before+1 {
		t.Errorf("%d commits", got-before)
	}
	if s := taskOf(t, ts, a.ULID).Sprint; s != nil {
		t.Errorf("task keeps sprint %v", s)
	}
	vs, err := planning.SprintList(ts.Session, "", "")
	if err != nil || len(vs) != 0 {
		t.Fatalf("list = %v, %v", vs, err)
	}
}

func TestSprintListCounts(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	s1 := addSprint(t, ts, "s1")
	ts.AddTask("a", func(task *model.Task) {
		inSprint(t, s1)(task)
		task.Status = "done"
		task.Props.Set("points", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "3"})
	})
	ts.AddTask("b", func(task *model.Task) {
		inSprint(t, s1)(task)
		task.Props.Set("points", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: "2.5"})
	})
	vs, err := planning.SprintList(ts.Session, "", model.SprintPlanned)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 || vs[0].Tasks != 2 || vs[0].Done != 1 || vs[0].Points != 5.5 {
		t.Fatalf("list = %+v", vs)
	}
	if vs, _ := planning.SprintList(ts.Session, "", model.SprintClosed); len(vs) != 0 {
		t.Fatalf("closed list = %+v", vs)
	}
}

func TestSprintNameAcrossWorkspaces(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("docs", "DOC")
	wss := []string{"docs", "me"}
	for _, ws := range wss {
		if _, err := planning.SprintAdd(ts.Session, planning.SprintAddInput{Name: "s1", Workspace: ws, Start: "2026-11-02", End: "2026-11-13"}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := planning.SprintStart(ts.Session, "s1", "")
	usecasetest.WantErr(t, err, usecase.KindAmbiguous, "sprint.ambiguous")
	w, err := planning.SprintStart(ts.Session, "s1", "docs")
	if err != nil {
		t.Fatal(err)
	}
	if w.Sprint.Workspace.Name != "docs" {
		t.Fatalf("workspace = %+v", w.Sprint.Workspace)
	}
}
