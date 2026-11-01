package ops

import (
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestProgressOf(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addTask("P", func(x *model.Task) { x.Kind = "project" })
	e := ts.addTask("E", func(x *model.Task) { x.Kind = "epic"; x.Parent = &p.ULID; x.Status = "done" })
	ts.addTask("a", func(x *model.Task) { x.Parent = &e.ULID; x.Status = "done" })
	ts.addTask("b", func(x *model.Task) { x.Parent = &e.ULID; x.Status = "in_progress" })
	ts.addTask("c", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "canceled" })
	d := ts.addTask("d", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "done" })
	ts.addTask("gone", func(x *model.Task) {
		x.Parent = &p.ULID
		now := ts.Now()
		x.DeletedAt = &now
	})
	ts.addTask("sub", func(x *model.Task) { x.Parent = &d.ULID; x.Kind = "task"; x.Status = "todo" })
	gdel := ts.addTask("gdel", func(x *model.Task) {
		x.Parent = &p.ULID
		now := ts.Now()
		x.DeletedAt = &now
	})
	ts.addTask("kid", func(x *model.Task) { x.Parent = &gdel.ULID; x.Status = "todo" })
	empty := ts.addTask("Empty", func(x *model.Task) { x.Kind = "project" })
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	get := func(u model.ULID) *Count {
		en, _ := ix.Task(u)
		return ProgressOf(ix, en)
	}
	// work descendants of P: a (done), b, c (canceled, left out), d (done), sub, kid (under the deleted gdel, still counted); gone and gdel are deleted
	if got := get(p.ULID); got == nil || *got != (Count{Done: 2, Total: 5}) {
		t.Errorf("P = %+v; want 2/5", got)
	}
	if got := get(e.ULID); got == nil || *got != (Count{Done: 1, Total: 2}) {
		t.Errorf("E = %+v; want 1/2", got)
	}
	if got := get(empty.ULID); got == nil || *got != (Count{}) {
		t.Errorf("Empty = %+v; want 0/0", got)
	}
	if got := get(d.ULID); got != nil {
		t.Errorf("a task of the work role has no progress: %+v", got)
	}
}

func TestChecklistOfAndText(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	x := ts.addTask("x", func(x *model.Task) { x.Body = "- [x] a\n- [ ] b\n" })
	y := ts.addTask("y", nil)
	_, ix, _ := ts.Open()
	ex, _ := ix.Task(x.ULID)
	ey, _ := ix.Task(y.ULID)
	if got := ChecklistOf(ex); got == nil || *got != (Count{Done: 1, Total: 2}) {
		t.Errorf("x = %+v", got)
	}
	if got := ChecklistOf(ey); got != nil {
		t.Errorf("y = %+v; want nil", got)
	}
	for _, c := range []struct {
		c, p *Count
		want string
	}{
		{nil, nil, ""},
		{&Count{1, 2}, nil, "☑ 1/2"},
		{nil, &Count{3, 14}, "3/14"},
		{&Count{2, 5}, &Count{3, 14}, "☑ 2/5  3/14"},
	} {
		if got := ProgressText(c.c, c.p); got != c.want {
			t.Errorf("ProgressText(%v, %v) = %q; want %q", c.c, c.p, got, c.want)
		}
	}
}

func TestViewHasChecklistAndProgress(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addTask("P", func(x *model.Task) { x.Kind = "project"; x.Body = "- [x] a\n- [ ] b\n- [ ] c\n" })
	ts.addTask("w", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "done" })
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(p.ULID)
	j := TreeViewOf(ix, &TreeNode{Entry: e})
	if j.Checklist == nil || *j.Checklist != (Count{Done: 1, Total: 3}) || j.Progress == nil || *j.Progress != (Count{Done: 1, Total: 1}) {
		t.Errorf("checklist = %+v, progress = %+v; want 1/3 and 1/1", j.Checklist, j.Progress)
	}
	sv := ShowViewOf(ShowResult{Index: ix, Entry: e})
	if sv.Checklist == nil || sv.Progress == nil {
		t.Errorf("show view: checklist = %+v, progress = %+v; want both", sv.Checklist, sv.Progress)
	}
}
