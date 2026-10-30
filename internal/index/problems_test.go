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

func ptr(id model.ID) *model.ID { return &id }

func TestProblems(t *testing.T) {
	a, b, c, d := model.NewID(), model.NewID(), model.NewID(), model.NewID()
	x, y, z, self := model.NewID(), model.NewID(), model.NewID(), model.NewID()
	missing := model.NewID()
	ix := indexOf(
		&Entry{ID: a, Broken: "bad yaml"},
		&Entry{ID: b, Violations: []schema.Violation{{Code: schema.CodeUnknownKind, Field: "kind", Msg: "m"}}},
		&Entry{ID: c, Parent: ptr(missing), BlockedBy: []model.ID{missing}, Props: []Prop{{Key: "duplicates", Values: []string{missing.String(), b.String()}}}},
		&Entry{ID: d, Parent: ptr(x)}, // points into the cycle, is not on it
		&Entry{ID: x, Parent: ptr(y)},
		&Entry{ID: y, Parent: ptr(x), BlockedBy: []model.ID{z}},
		&Entry{ID: z, BlockedBy: []model.ID{y}},
		&Entry{ID: self, BlockedBy: []model.ID{self}},
	)
	tests := []struct {
		id   model.ID
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
		if got := problemCodes(ix.Problems(tt.id)); !slices.Equal(got, tt.want) {
			t.Errorf("problems of %s = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestReverse(t *testing.T) {
	a, b, c := model.NewID(), model.NewID(), model.NewID()
	ix := indexOf(
		&Entry{ID: a, Title: "a", BlockedBy: []model.ID{c}, Props: []Prop{
			{Key: "duplicates", Values: []string{c.String()}},
			{Key: "relates", Values: []string{b.String()}},
		}},
		&Entry{ID: b, Title: "b", BlockedBy: []model.ID{c}, Props: []Prop{{Key: "relates", Values: []string{a.String()}}}},
		&Entry{ID: c, Title: "c", Props: []Prop{{Key: "relates", Values: []string{a.String()}}}},
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
	a, b := model.NewID(), model.NewID()
	e := &Entry{BlockedBy: []model.ID{a}, Props: []Prop{{Key: "duplicates", Values: []string{b.String(), "not an id"}}}}
	if got := e.Targets("blocked_by"); !slices.Equal(got, []model.ID{a}) {
		t.Errorf("Targets(blocked_by) = %v", got)
	}
	if got := e.Targets("duplicates"); !slices.Equal(got, []model.ID{b}) {
		t.Errorf("Targets(duplicates) = %v", got)
	}
}

func TestKeyAndWorkspaceProblems(t *testing.T) {
	s, dir := newStore(t)
	ws := firstWorkspace(t, s)
	a := newTask("a")
	put(t, s, a) // TST-1
	b := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(b), rawTask(b, "key: TST-1\nworkspace: "+ws.String()+"\nkind: task\ntitle: b\nstatus: todo\n"))
	c := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(c), rawTask(c, "kind: task\ntitle: c\nstatus: todo\n"))
	d := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(d), rawTask(d, "key: XX-1\nworkspace: "+ws.String()+"\nkind: task\ntitle: d\nstatus: todo\n"))
	e := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(e), rawTask(e, "key: TST-9\nworkspace: "+model.NewID().String()+"\nkind: task\ntitle: e\nstatus: todo\n"))
	f := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(f), rawTask(f, "key: TST-10\nworkspace: "+ws.String()+"\nparent: "+e.String()+"\nkind: task\ntitle: f\nstatus: todo\n"))
	ix := mustOpen(t, s)
	has := func(id model.ID, code string) bool {
		return slices.ContainsFunc(ix.Problems(id), func(p Problem) bool { return p.Code == code })
	}
	if !has(a.ID, CodeDuplicateKey) || !has(b, CodeDuplicateKey) {
		t.Error("duplicate keys are not found")
	}
	if !has(c, CodeNoKey) || !has(c, store.CodeNoWorkspace) {
		t.Error("a task without key and workspace is not found")
	}
	if !has(d, CodeKeyPrefix) {
		t.Error("a key with another prefix is not found")
	}
	if !has(e, store.CodeNoWorkspace) {
		t.Error("a task in a workspace that is not in the store is not found")
	}
	if !has(f, store.CodeWorkspaceMismatch) {
		t.Error("a task in another workspace than its parent is not found")
	}
	if has(a.ID, CodeNoKey) || has(a.ID, store.CodeNoWorkspace) || has(a.ID, CodeKeyPrefix) {
		t.Errorf("problems of a good task = %v", ix.Problems(a.ID))
	}
}
