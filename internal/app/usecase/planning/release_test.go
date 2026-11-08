package planning_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/planning"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func addRelease(t *testing.T, ts *usecasetest.Session, name string) model.ULID {
	t.Helper()
	w, err := planning.ReleaseAdd(ts.Session, name, "2026-12-01", "")
	if err != nil {
		t.Fatal(err)
	}
	u, err := model.ParseULID(w.Release.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestReleaseAddAndSet(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	addRelease(t, ts, "v1")
	addRelease(t, ts, "v2")
	_, err := planning.ReleaseAdd(ts.Session, "V1", "", "")
	usecasetest.WantErr(t, err, usecase.KindConflict, "release.name_taken")
	_, err = planning.ReleaseAdd(ts.Session, "none", "", "")
	usecasetest.WantErr(t, err, usecase.KindUsage, "release.bad_name")

	assign := func(s string) []usecase.Assign {
		as, err := usecase.ParseAssigns([]string{s})
		if err != nil {
			t.Fatal(err)
		}
		return as
	}
	for _, state := range []string{model.ReleaseReleased, model.ReleaseArchived, model.ReleaseUnreleased} {
		w, err := planning.ReleaseSet(ts.Session, "v1", "", assign("state="+state))
		if err != nil {
			t.Fatal(err)
		}
		if w.Release.State != state {
			t.Fatalf("state = %s, want %s", w.Release.State, state)
		}
	}
	_, err = planning.ReleaseSet(ts.Session, "v1", "", assign("state=shipped"))
	usecasetest.WantErr(t, err, usecase.KindInvalid, "assign.bad_value")
	_, err = planning.ReleaseSet(ts.Session, "v1", "", assign("name=v2"))
	usecasetest.WantErr(t, err, usecase.KindConflict, "release.name_taken")
	_, err = planning.ReleaseSet(ts.Session, "v1", "", assign("goal=x"))
	usecasetest.WantErr(t, err, usecase.KindInvalid, "assign.unknown_key")
	w, err := planning.ReleaseSet(ts.Session, "v1", "", assign("date=2027-01-01"))
	if err != nil || w.Release.Date != "2027-01-01" {
		t.Fatalf("write = %+v, %v", w, err)
	}
}

func TestReleaseNotes(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	v1 := addRelease(t, ts, "v1")
	in := func(kind, status string) func(*model.Task) {
		return func(task *model.Task) {
			task.Kind, task.Status, task.Releases = kind, status, []model.ULID{v1}
		}
	}
	a := ts.AddTask("first", in("task", "done"))
	b := ts.AddTask("second", in("task", "done"))
	ts.AddTask("not done", in("task", "todo"))
	ts.AddTask("elsewhere", func(task *model.Task) { task.Status = "done" })
	e := ts.AddTask("big", in("epic", "done"))

	got, err := planning.ReleaseNotes(ts.Session, "v1", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "# v1\n\n## Epic\n\n- " + e.ID + " big\n\n## Task\n\n- " + a.ID + " first\n- " + b.ID + " second\n"
	if got != want {
		t.Fatalf("notes:\n%q\nwant:\n%q", got, want)
	}

	vs, err := planning.ReleaseList(ts.Session, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 || vs[0].Tasks != 4 || vs[0].Done != 3 {
		t.Fatalf("list = %+v", vs)
	}
}

func TestReleaseRemoveTakesItOffTasks(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	v1, v2 := addRelease(t, ts, "v1"), addRelease(t, ts, "v2")
	a := ts.AddTask("a", func(task *model.Task) { task.Releases = []model.ULID{v1, v2} })
	if _, err := planning.ReleaseRemove(ts.Session, "v1", ""); err != nil {
		t.Fatal(err)
	}
	if got := taskOf(t, ts, a.ULID).Releases; len(got) != 1 || got[0] != v2 {
		t.Fatalf("releases = %v", got)
	}
}
