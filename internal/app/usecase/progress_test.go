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
	ts.AddTask("b", func(x *model.Task) { x.Parent = &e.ULID; x.Status = "in_progress" })
	ts.AddTask("c", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "canceled" })
	d := ts.AddTask("d", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "done" })
	ts.AddTask("gone", func(x *model.Task) {
		x.Parent = &p.ULID
		now := ts.Now()
		x.DeletedAt = &now
	})
	ts.AddTask("sub", func(x *model.Task) { x.Parent = &d.ULID; x.Kind = "task"; x.Status = "todo" })
	gdel := ts.AddTask("gdel", func(x *model.Task) {
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
	// work descendants of P: a (done), b, c (canceled, left out), d (done), sub, kid (under the deleted gdel, still counted); gone and gdel are deleted
	if got := get(p.ULID); got == nil || *got != (usecase.Count{Done: 2, Total: 5}) {
		t.Errorf("P = %+v; want 2/5", got)
	}
	if got := get(e.ULID); got == nil || *got != (usecase.Count{Done: 1, Total: 2}) {
		t.Errorf("E = %+v; want 1/2", got)
	}
	if got := get(empty.ULID); got == nil || *got != (usecase.Count{}) {
		t.Errorf("Empty = %+v; want 0/0", got)
	}
	if got := get(d.ULID); got != nil {
		t.Errorf("a task of the work role has no progress: %+v", got)
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
	j := browse.TreeViewOf(ix, &browse.TreeNode{Entry: e})
	if j.Checklist == nil || *j.Checklist != (usecase.Count{Done: 1, Total: 3}) || j.Progress == nil || *j.Progress != (usecase.Count{Done: 1, Total: 1}) {
		t.Errorf("checklist = %+v, progress = %+v; want 1/3 and 1/1", j.Checklist, j.Progress)
	}
	sv := browse.ShowViewOf(browse.ShowResult{Index: ix, Entry: e})
	if sv.Checklist == nil || sv.Progress == nil {
		t.Errorf("show view: checklist = %+v, progress = %+v; want both", sv.Checklist, sv.Progress)
	}
}
