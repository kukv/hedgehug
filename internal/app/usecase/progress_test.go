package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestProgressOf(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddTask("P", func(x *model.Task) { x.Kind = "project" })
	e := ts.AddTask("E", func(x *model.Task) { x.Kind = "epic"; x.Parent = &p.ULID; x.Status = "done" })
	ts.AddTask("a", func(x *model.Task) { x.Parent = &e.ULID; x.Status = "done" })
	ts.AddTask("b", func(x *model.Task) { x.Kind = "bug"; x.Parent = &e.ULID; x.Status = "in_progress" })
	ts.AddTask("c", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "canceled" })
	d := ts.AddTask("d", func(x *model.Task) { x.Kind = "story"; x.Parent = &p.ULID; x.Status = "done" })
	ts.AddTask("gone", func(x *model.Task) {
		x.Parent = &p.ULID
		now := ts.Now()
		x.DeletedAt = &now
	})
	ts.AddTask("sub", func(x *model.Task) { x.Parent = &d.ULID; x.Kind = "step"; x.Status = "todo" })
	gdel := ts.AddTask("gdel", func(x *model.Task) {
		x.Kind = "epic"
		x.Parent = &p.ULID
		now := ts.Now()
		x.DeletedAt = &now
	})
	ts.AddTask("kid", func(x *model.Task) { x.Parent = &gdel.ULID; x.Status = "todo" })
	empty := ts.AddTask("Empty", func(x *model.Task) { x.Kind = "project" })
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	get := func(u model.ULID) *usecase.Count {
		en, _ := ix.Task(u)
		return usecase.ProgressOf(ix, en)
	}
	// stories, tasks and bugs under P: a (done), b, c (canceled, left out), d (done), kid (under the deleted epic gdel, still counted); gone is deleted; sub is a step, which is not counted
	if got := get(p.ULID); got == nil || *got != (usecase.Count{Done: 2, Total: 4}) {
		t.Errorf("P = %+v; want 2/4", got)
	}
	if got := get(e.ULID); got == nil || *got != (usecase.Count{Done: 1, Total: 2}) {
		t.Errorf("E = %+v; want 1/2", got)
	}
	if got := get(empty.ULID); got == nil || *got != (usecase.Count{}) {
		t.Errorf("Empty = %+v; want 0/0", got)
	}
	if got := get(d.ULID); got != nil {
		t.Errorf("a story has no progress: %+v", got)
	}
}

func TestStepsOfBreaksDownTheStepsOfAnItem(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddTask("P", func(x *model.Task) { x.Kind = "project" })
	e := ts.AddTask("E", func(x *model.Task) { x.Kind = "epic"; x.Parent = &p.ULID })
	s1 := ts.AddTask("S1", func(x *model.Task) { x.Kind = "story"; x.Parent = &e.ULID; x.Status = "done" })
	ts.AddTask("S2", func(x *model.Task) { x.Kind = "story"; x.Parent = &e.ULID })
	for _, st := range []string{"done", "done", "todo", "canceled"} {
		ts.AddTask("step "+st, func(x *model.Task) { x.Kind = "step"; x.Parent = &s1.ULID; x.Status = st })
	}
	ts.AddTask("gone", func(x *model.Task) {
		x.Kind = "step"
		x.Parent = &s1.ULID
		now := ts.Now()
		x.DeletedAt = &now
	})
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	steps := func(u model.ULID) *usecase.Count {
		en, _ := ix.Task(u)
		return usecase.StepsOf(ix, en)
	}
	// the epic counts the two stories, not the steps: 1 of 2
	if en, _ := ix.Task(e.ULID); *usecase.ProgressOf(ix, en) != (usecase.Count{Done: 1, Total: 2}) {
		t.Errorf("E = %+v; want 1/2", usecase.ProgressOf(ix, en))
	}
	// the story shows its steps, 2 of 3 (the canceled one and the deleted one are left out)
	if got := steps(s1.ULID); got == nil || *got != (usecase.Count{Done: 2, Total: 3}) {
		t.Errorf("S1 steps = %+v; want 2/3", got)
	}
	for _, u := range []model.ULID{p.ULID, e.ULID} {
		if got := steps(u); got != nil {
			t.Errorf("a group has no steps: %+v", got)
		}
	}
	en, _ := ix.Task(s1.ULID)
	if v := usecase.ListViewOf(ix, en); v.Steps == nil || *v.Steps != (usecase.Count{Done: 2, Total: 3}) || v.Progress != nil {
		t.Errorf("view: steps %+v progress %+v", v.Steps, v.Progress)
	}
}

func TestChecklistOfAndText(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	x := ts.AddTask("x", func(x *model.Task) { x.Body = "- [x] a\n- [ ] b\n" })
	y := ts.AddTask("y", nil)
	_, ix, _ := ts.Open()
	ex, _ := ix.Task(x.ULID)
	ey, _ := ix.Task(y.ULID)
	if got := usecase.ChecklistOf(ex); got == nil || *got != (usecase.Count{Done: 1, Total: 2}) {
		t.Errorf("x = %+v", got)
	}
	if got := usecase.ChecklistOf(ey); got != nil {
		t.Errorf("y = %+v; want nil", got)
	}
	for _, c := range []struct {
		c, p *usecase.Count
		want string
	}{
		{nil, nil, ""},
		{&usecase.Count{1, 2}, nil, "☑ 1/2"},
		{nil, &usecase.Count{3, 14}, "3/14"},
		{&usecase.Count{2, 5}, &usecase.Count{3, 14}, "☑ 2/5  3/14"},
	} {
		if got := usecase.ProgressText(c.c, c.p); got != c.want {
			t.Errorf("ProgressText(%v, %v) = %q; want %q", c.c, c.p, got, c.want)
		}
	}
	for _, c := range []struct {
		c    *usecase.Count
		want string
	}{
		{nil, ""},
		{&usecase.Count{0, 0}, "☑ 0/0"},
		{&usecase.Count{1, 3}, "☑ 1/3"},
	} {
		if got := usecase.ChecklistText(c.c); got != c.want {
			t.Errorf("ChecklistText(%v) = %q; want %q", c.c, got, c.want)
		}
	}
	for _, c := range []struct {
		c    *usecase.Count
		want string
	}{
		{nil, ""},
		{&usecase.Count{0, 0}, "0/0"},
		{&usecase.Count{1, 2}, "1/2"},
	} {
		if got := usecase.ProgressFractionText(c.c); got != c.want {
			t.Errorf("ProgressFractionText(%v) = %q; want %q", c.c, got, c.want)
		}
	}
}

func TestViewHasChecklistAndProgress(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddTask("P", func(x *model.Task) { x.Kind = "project"; x.Body = "- [x] a\n- [ ] b\n- [ ] c\n" })
	ts.AddTask("w", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "done" })
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(p.ULID)
	j := usecase.ListViewOf(ix, e)
	if j.Checklist == nil || *j.Checklist != (usecase.Count{Done: 1, Total: 3}) || j.Progress == nil || *j.Progress != (usecase.Count{Done: 1, Total: 1}) {
		t.Errorf("checklist = %+v, progress = %+v; want 1/3 and 1/1", j.Checklist, j.Progress)
	}
	sv, err := browse.Show(ts.Session, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sv.Checklist == nil || sv.Progress == nil {
		t.Errorf("show view: checklist = %+v, progress = %+v; want both", sv.Checklist, sv.Progress)
	}
}
