package index

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
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
