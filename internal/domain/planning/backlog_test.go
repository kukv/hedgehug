package planning_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// wi は、ワークスペース W の作業項目(Story)。
func wi(id item.ID, rk string) item.Item {
	return item.Item{ID: id, Place: item.Place{Workspace: "W"}, Kind: item.Story, Rank: rk, Status: "Todo"}
}

func ids(its []item.Item) []item.ID {
	var out []item.ID
	for _, it := range its {
		out = append(out, it.ID)
	}
	return out
}

func sameIDs(t *testing.T, label string, its []item.Item, want ...item.ID) {
	t.Helper()
	got := ids(its)
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}

func TestBacklogFrames(t *testing.T) {
	deletion := &change.Deletion{Op: "x", At: jst(2, 10, 0), Root: true}
	plan := planning.Plan{Sprints: []planning.Sprint{
		{ID: "S0", Name: "Sprint 0", State: planning.Closed, Start: "2026-10-01", End: "2026-10-14"},
		{ID: "S1", Name: "Sprint 1", State: planning.Active, Start: "2026-11-02", End: "2026-11-15"},
		{ID: "S2", Name: "Sprint 2", State: planning.Planned, Start: "2026-11-20", End: "2026-12-03"},
		{ID: "S3", Name: "Sprint 3", State: planning.Planned},
		{ID: "S4", Name: "Sprint 4", State: planning.Planned, Deleted: deletion},
	}}
	a, b, c, d, e, f := wi("A", "c"), wi("B", "b"), wi("C", "a"), wi("D", "d"), wi("E", "e"), wi("F", "f")
	a.Sprint, a.Status, a.Points = "S1", "Done", pt(3)
	b.Sprint, b.Status, b.Points = "S1", "Canceled", pt(5)
	c.Sprint = "S1"
	d.Sprint = "S4"
	e.Status = "Done"
	g := wi("G", "g")
	g.Kind = item.Epic
	// S1 の A・B・C は順位を逆に付け、並びが引数の順でなく WorkOrder(C・B・A)になることを見る。
	frames := planning.BacklogFrames([]item.Item{a, b, c, d, e, f, g}, "W", plan, settings())
	if len(frames) != 4 {
		t.Fatalf("枠 = %d, want 4", len(frames))
	}
	for i, id := range []change.ID{"S1", "S2", "S3"} {
		if frames[i].Sprint == nil || frames[i].Sprint.ID != id {
			t.Fatalf("枠 %d = %+v, want %s", i, frames[i].Sprint, id)
		}
	}
	sameIDs(t, "S1", frames[0].Items, "C", "B", "A")
	if frames[0].Total != (planning.Totals{Points: 3, Count: 2}) {
		t.Fatalf("S1 の Total = %+v", frames[0].Total)
	}
	sameIDs(t, "S2", frames[1].Items)
	if frames[3].Sprint != nil {
		t.Fatalf("最後はバックログの枠: %+v", frames[3].Sprint)
	}
	sameIDs(t, "バックログ", frames[3].Items, "D", "F")
	if frames[3].Total.Count != 2 {
		t.Fatalf("バックログの Total = %+v", frames[3].Total)
	}
}

func TestEndRank(t *testing.T) {
	its := []item.Item{wi("A", "X")}
	if got := planning.EndRank("V", its, "W", item.Story); got != rank.After("X") {
		t.Fatalf("Story = %q, want %q", got, rank.After("X"))
	}
	if got := planning.EndRank("V", its, "W", item.Step); got != rank.After("V") {
		t.Fatalf("Step = %q, want %q", got, rank.After("V"))
	}
	if got := planning.EndRank("Z", its, "W", item.Story); got != rank.After("Z") {
		t.Fatalf("親の子の末尾が大きい Story = %q, want %q", got, rank.After("Z"))
	}
	if got := planning.EndRank("", nil, "W", item.Bug); got != rank.First {
		t.Fatalf("空 = %q, want %q", got, rank.First)
	}
}

func TestPlacement(t *testing.T) {
	its := []item.Item{wi("A", "V"), wi("B", "W"), wi("C", "X")}
	for name, tc := range map[string]struct {
		moving, anchor item.ID
		after          bool
		want           string
	}{
		"C を A の直前": {"C", "A", false, rank.Between("", "V")},
		"A を C の直後": {"A", "C", true, rank.After("X")},
		"A を B の直後": {"A", "B", true, rank.Between("W", "X")},
	} {
		got, ok := planning.Placement(its, "W", tc.moving, tc.anchor, tc.after)
		if !ok || got != tc.want {
			t.Errorf("%s = %q, %v, want %q", name, got, ok, tc.want)
		}
	}
}

func TestPlacementTies(t *testing.T) {
	b, c := wi("B", "W"), wi("C", "W")
	b.Created.At = jst(1, 10, 0)
	c.Created.At = jst(1, 11, 0)
	its := []item.Item{wi("A", "V"), b, c, wi("F", "Y")}
	if got, ok := planning.Placement(its, "W", "C", "A", false); !ok || got != rank.Between("", "V") {
		t.Errorf("C を A の直前 = %q, %v", got, ok)
	}
	if got, ok := planning.Placement(its, "W", "A", "C", true); !ok || got != rank.After("W") {
		t.Errorf("A を C の直後 = %q, %v, want %q", got, ok, rank.After("W"))
	}
	got, ok := planning.Placement(its, "W", "F", "C", false)
	if !ok || got != rank.Between("V", "W") || got <= "V" || got >= "W" {
		t.Errorf("F を C の直前 = %q, %v, want V と W の間", got, ok)
	}
}

func TestPlacementNotInOrder(t *testing.T) {
	epic := wi("E", "W")
	epic.Kind = item.Epic
	gone := wi("G", "X")
	gone.Deleted = &change.Deletion{Op: "x", At: jst(2, 10, 0), Root: true}
	other := wi("O", "Y")
	other.Place.Workspace = "W2"
	its := []item.Item{wi("A", "V"), wi("M", "Z"), epic, gone, other}
	for _, anchor := range []item.ID{"E", "G", "O", "none", "M"} {
		if got, ok := planning.Placement(its, "W", "M", anchor, false); ok {
			t.Errorf("anchor %s = %q, want false", anchor, got)
		}
	}
}

func TestAppendWorks(t *testing.T) {
	a := wi("A", "Y")
	step := wi("S", "V")
	step.Kind = item.Step
	b := wi("B", "W")
	b.Kind = item.Task
	got := planning.AppendWorks([]item.Item{a, step, b}, "X")
	if len(got) != 2 {
		t.Fatalf("結果 = %v, want 2 件", got)
	}
	if got["B"] != rank.After("X") || got["A"] != rank.After(rank.After("X")) {
		t.Fatalf("結果 = %v", got)
	}
}
