package index

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func problemCodes(ps []Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Code+":"+p.Field)
	}
	slices.Sort(out)
	return out
}

func ptr(ulid model.ULID) *model.ULID { return &ulid }

func TestProblems(t *testing.T) {
	a, b, c, d := model.NewULID(), model.NewULID(), model.NewULID(), model.NewULID()
	x, y, z, self := model.NewULID(), model.NewULID(), model.NewULID(), model.NewULID()
	missing := model.NewULID()
	ix := indexOf(
		&Entry{ULID: a, Broken: "bad yaml"},
		&Entry{ULID: b, Violations: []schema.Violation{{Code: schema.CodeUnknownKind, Field: "kind", Msg: "m"}}},
		&Entry{ULID: c, Parent: ptr(missing), BlockedBy: []model.ULID{missing}, Props: []Prop{{Key: "duplicates", Values: []string{missing.String(), b.String()}}}},
		&Entry{ULID: d, Parent: ptr(x)}, // points into the cycle, is not on it
		&Entry{ULID: x, Parent: ptr(y)},
		&Entry{ULID: y, Parent: ptr(x), BlockedBy: []model.ULID{z}},
		&Entry{ULID: z, BlockedBy: []model.ULID{y}},
		&Entry{ULID: self, BlockedBy: []model.ULID{self}},
	)
	tests := []struct {
		ulid model.ULID
		want []string
	}{
		{a, []string{"broken:"}},
		{b, []string{"unknown_kind:kind"}},
		{c, []string{"missing_ref:blocked_by", "missing_ref:duplicates", "missing_ref:parent"}},
		{d, nil},
		{x, []string{"cycle:parent"}},
		{y, []string{"cycle:blocked_by", "cycle:parent"}},
		{z, []string{"cycle:blocked_by"}},
		{self, []string{"cycle:blocked_by"}},
	}
	for _, tt := range tests {
		if got := problemCodes(ix.Problems(tt.ulid)); !slices.Equal(got, tt.want) {
			t.Errorf("problems of %s = %v, want %v", tt.ulid, got, tt.want)
		}
	}
}

func TestReverse(t *testing.T) {
	a, b, c := model.NewULID(), model.NewULID(), model.NewULID()
	ix := indexOf(
		&Entry{ULID: a, Title: "a", BlockedBy: []model.ULID{c}, Props: []Prop{
			{Key: "duplicates", Values: []string{c.String()}},
			{Key: "relates", Values: []string{b.String()}},
		}},
		&Entry{ULID: b, Title: "b", BlockedBy: []model.ULID{c}, Props: []Prop{{Key: "relates", Values: []string{a.String()}}}},
		&Entry{ULID: c, Title: "c", Props: []Prop{{Key: "relates", Values: []string{a.String()}}}},
	)
	want := func(es []*Entry, titles ...string) {
		t.Helper()
		var got []string
		for _, e := range es {
			got = append(got, e.Title)
		}
		slices.Sort(got)
		if !slices.Equal(got, titles) {
			t.Errorf("got %v, want %v", got, titles)
		}
	}
	want(ix.Reverse(c, "blocked_by"), "a", "b")
	want(ix.Reverse(c, "duplicates"), "a")
	want(ix.Reverse(a, "duplicates"))
	// relates is symmetric: both directions, each task once.
	want(ix.Reverse(a, "relates"), "b", "c")
	want(ix.Reverse(c, "relates"), "a")
}

func TestTargets(t *testing.T) {
	a, b := model.NewULID(), model.NewULID()
	e := &Entry{BlockedBy: []model.ULID{a}, Props: []Prop{{Key: "duplicates", Values: []string{b.String(), "not a ulid"}}}}
	if got := e.Targets("blocked_by"); !slices.Equal(got, []model.ULID{a}) {
		t.Errorf("Targets(blocked_by) = %v", got)
	}
	if got := e.Targets("duplicates"); !slices.Equal(got, []model.ULID{b}) {
		t.Errorf("Targets(duplicates) = %v", got)
	}
}

func TestIDAndWorkspaceProblems(t *testing.T) {
	s, dir := newStore(t)
	ws := firstWorkspace(t, s)
	a := newTask("a")
	put(t, s, a) // TST-1
	b := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(b), rawTask(b, "id: TST-1\nworkspace: "+ws.String()+"\nkind: task\ntitle: b\nstatus: todo\n"))
	c := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(c), rawTask(c, "kind: task\ntitle: c\nstatus: todo\n"))
	d := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(d), rawTask(d, "id: XX-1\nworkspace: "+ws.String()+"\nkind: task\ntitle: d\nstatus: todo\n"))
	e := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(e), rawTask(e, "id: TST-9\nworkspace: "+model.NewULID().String()+"\nkind: task\ntitle: e\nstatus: todo\n"))
	f := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(f), rawTask(f, "id: TST-10\nworkspace: "+ws.String()+"\nparent: "+e.String()+"\nkind: task\ntitle: f\nstatus: todo\n"))
	ix := mustOpen(t, s)
	has := func(ulid model.ULID, code string) bool {
		return slices.ContainsFunc(ix.Problems(ulid), func(p Problem) bool { return p.Code == code })
	}
	if !has(a.ULID, CodeDuplicateID) || !has(b, CodeDuplicateID) {
		t.Error("duplicate ids are not found")
	}
	if !has(c, CodeNoID) || !has(c, store.CodeNoWorkspace) {
		t.Error("a task without id and workspace is not found")
	}
	if !has(d, CodeIDPrefix) {
		t.Error("an id with another prefix is not found")
	}
	if !has(e, store.CodeNoWorkspace) {
		t.Error("a task in a workspace that is not in the store is not found")
	}
	if !has(f, store.CodeWorkspaceMismatch) {
		t.Error("a task in another workspace than its parent is not found")
	}
	if has(a.ULID, CodeNoID) || has(a.ULID, store.CodeNoWorkspace) || has(a.ULID, CodeIDPrefix) {
		t.Errorf("problems of a good task = %v", ix.Problems(a.ULID))
	}
}

func TestParentDeletedProblem(t *testing.T) {
	s, dir := newStore(t)
	ws := firstWorkspace(t, s)
	p, live, gone := model.NewULID(), model.NewULID(), model.NewULID()
	common := "workspace: " + ws.String() + "\nkind: task\nstatus: todo\n"
	commitRaw(t, s, dir, model.TaskPath(p), rawTask(p, "id: TST-1\n"+common+"title: p\ndeleted_at: 2026-10-30T12:00:00Z\n"))
	commitRaw(t, s, dir, model.TaskPath(live), rawTask(live, "id: TST-2\nparent: "+p.String()+"\n"+common+"title: live\n"))
	commitRaw(t, s, dir, model.TaskPath(gone), rawTask(gone, "id: TST-3\nparent: "+p.String()+"\n"+common+"title: gone\ndeleted_at: 2026-10-30T12:00:00Z\n"))
	ix := mustOpen(t, s)
	has := func(u model.ULID) bool {
		return slices.ContainsFunc(ix.Problems(u), func(x Problem) bool { return x.Code == CodeParentDeleted && x.Field == "parent" })
	}
	if !has(live) || has(gone) || has(p) {
		t.Errorf("problems: live %v, deleted child %v, parent %v", ix.Problems(live), ix.Problems(gone), ix.Problems(p))
	}
}
