package planning_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

func TestWorkOrder(t *testing.T) {
	at := func(m int) item.Stamp { return item.Stamp{At: time.Date(2026, 11, 1, 0, m, 0, 0, time.UTC)} }
	in := []item.Item{
		{ID: "d", Rank: "B", Created: at(5)},
		{ID: "c", Rank: "B", Created: at(1)},
		{ID: "b", Rank: "B", Created: at(1)},
		{ID: "a", Rank: "A", Created: at(9)},
	}
	var got []item.ID
	for _, it := range planning.WorkOrder(in) {
		got = append(got, it.ID)
	}
	if want := []item.ID{"a", "b", "c", "d"}; !slices.Equal(got, want) {
		t.Fatalf("WorkOrder = %v, want %v", got, want)
	}
	if in[0].ID != "d" || in[3].ID != "a" {
		t.Fatal("入力を並べ替えない")
	}
}

func names(ss []planning.Sprint) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Name)
	}
	return out
}

func TestScreenOrder(t *testing.T) {
	in := []planning.Sprint{
		{ID: "1", Name: "c-old", State: planning.Closed, End: "2026-10-01"},
		{ID: "2", Name: "p-undecided-b", State: planning.Planned},
		{ID: "3", Name: "p-late", State: planning.Planned, Start: "2026-12-01", End: "2026-12-14"},
		{ID: "4", Name: "c-new", State: planning.Closed, End: "2026-11-01"},
		{ID: "5", Name: "p-same-b", State: planning.Planned, Start: "2026-11-20"},
		{ID: "6", Name: "a", State: planning.Active, Start: "2026-11-01", End: "2026-11-14"},
		{ID: "7", Name: "p-same-a", State: planning.Planned, Start: "2026-11-20"},
		{ID: "8", Name: "p-undecided-a", State: planning.Planned},
		{ID: "9", Name: "c-new-a", State: planning.Closed, End: "2026-11-01"},
	}
	orig := slices.Clone(in)
	want := []string{"a", "p-same-a", "p-same-b", "p-late", "p-undecided-a", "p-undecided-b", "c-new", "c-new-a", "c-old"}
	if got := names(planning.ScreenOrder(in)); !slices.Equal(got, want) {
		t.Fatalf("ScreenOrder = %v, want %v", got, want)
	}
	if !slices.Equal(in, orig) {
		t.Fatal("入力を並べ替えない")
	}
	wantPlanned := []string{"p-same-a", "p-same-b", "p-late", "p-undecided-a", "p-undecided-b"}
	if got := names(planning.PlannedOrder(in)); !slices.Equal(got, wantPlanned) {
		t.Fatalf("PlannedOrder = %v, want %v", got, wantPlanned)
	}
}

func TestReleaseOrder(t *testing.T) {
	in := []planning.Release{
		{ID: "1", Name: "arch", State: planning.Archived, Date: "2026-01-01"},
		{ID: "2", Name: "u-nodate", State: planning.Unreleased},
		{ID: "3", Name: "r1", State: planning.Released, Date: "2026-02-01"},
		{ID: "4", Name: "u-late", State: planning.Unreleased, Date: "2026-12-01"},
		{ID: "5", Name: "u-same-b", State: planning.Unreleased, Date: "2026-11-20"},
		{ID: "6", Name: "u-same-a", State: planning.Unreleased, Date: "2026-11-20"},
		{ID: "7", Name: "r-nodate", State: planning.Released},
	}
	var got []string
	for _, r := range planning.ReleaseOrder(in) {
		got = append(got, r.Name)
	}
	want := []string{"u-same-a", "u-same-b", "u-late", "u-nodate", "r1", "r-nodate", "arch"}
	if !slices.Equal(got, want) {
		t.Fatalf("ReleaseOrder = %v, want %v", got, want)
	}
}
