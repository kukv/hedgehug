package planning_test

import (
	"math"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// closedWith は、開始日 start の完了したスプリントで、確定値の実績が done ポイント(件数は done の整数部)のもの。
func closedWith(id, name, start string, done float64) planning.Sprint {
	return planning.Sprint{
		ID: change.ID(id), Name: name, State: planning.Closed, Start: start, End: start,
		Confirmed: &planning.Confirmed{
			Committed: planning.Totals{Points: done + 1, Count: int(done) + 1},
			Completed: planning.Totals{Points: done, Count: int(done)},
		},
	}
}

func velocityIDs(bars []planning.VelocityBar) []change.ID {
	out := make([]change.ID, len(bars))
	for i, b := range bars {
		out[i] = b.Sprint.ID
	}
	return out
}

func TestVelocityClosedUsesConfirmed(t *testing.T) {
	localTokyo(t)
	a := inS1("A", pt(3))
	s := closedSprint(t, works(t, a))
	// 完了のあとにアイテムを変えても、棒は確定値のまま。
	later := works(t, setStatus(setSprint(a, jst(6, 9, 0), "S1", nil), jst(6, 9, 0), "Todo", "Done"))
	bars, _ := planning.Velocity(later, "W", []planning.Sprint{s}, settings(), planning.UnitPoints, jst(7, 12, 0))
	if len(bars) != 1 || bars[0].Planned != 3 || bars[0].Done != 0 {
		t.Fatalf("bars = %+v, want 計画 3・完了 0", bars)
	}
	bars, _ = planning.Velocity(later, "W", []planning.Sprint{s}, settings(), planning.UnitCount, jst(7, 12, 0))
	if bars[0].Planned != 1 || bars[0].Done != 0 {
		t.Fatalf("件数 bars = %+v", bars)
	}
	s.Confirmed = nil
	bars, avg := planning.Velocity(later, "W", []planning.Sprint{s}, settings(), planning.UnitPoints, jst(7, 12, 0))
	if bars[0].Planned != 0 || bars[0].Done != 0 || avg == nil || *avg != 0 {
		t.Fatalf("Confirmed が nil なら 0: bars = %+v, avg = %v", bars, avg)
	}
}

func TestVelocityActive(t *testing.T) {
	localTokyo(t)
	a := setStatus(inS1("A", pt(3)), jst(4, 10, 0), "Todo", "Done")
	b := inS1("B", pt(5))
	late := setStatus(setSprint(created(jst(1, 10, 0), item.Item{ID: "C", Status: "Todo", Points: pt(2)}), jst(3, 10, 0), nil, "S1"), jst(4, 11, 0), "Todo", "Done")
	// 今日(11/4)より後に完了にしたものは、今日の終わり時点では完了でない。
	next := setStatus(inS1("D", pt(7)), jst(5, 10, 0), "Todo", "Done")
	ws := works(t, a, b, late, next)
	now := jst(4, 12, 0)
	bars, avg := planning.Velocity(ws, "W", []planning.Sprint{burnSprint()}, settings(), planning.UnitPoints, now)
	if len(bars) != 1 || bars[0].Planned != 15 || bars[0].Done != 5 || avg != nil {
		t.Fatalf("bars = %+v, avg = %v, want 計画 15(開始日の範囲)・完了 5", bars, avg)
	}
	bars, _ = planning.Velocity(ws, "W", []planning.Sprint{burnSprint()}, settings(), planning.UnitCount, now)
	if bars[0].Planned != 3 || bars[0].Done != 2 {
		t.Fatalf("件数 bars = %+v", bars)
	}
}

func TestVelocityAverage(t *testing.T) {
	localTokyo(t)
	// 実行中のスプリントは最も新しく、完了 100 のアイテムを持つ。平均に含めると値が変わる。
	active := planning.Sprint{ID: "A", Name: "Active", State: planning.Active, Start: "2026-11-20", End: "2026-11-25"}
	ws := works(t, created(jst(1, 10, 0), item.Item{ID: "X", Sprint: "A", Status: "Done", Points: pt(100)}))
	now := jst(22, 12, 0)
	avg := func(done ...float64) *float64 {
		live := []planning.Sprint{active}
		for i, d := range done {
			id := string(rune('a' + i))
			live = append(live, closedWith(id, id, "2026-10-0"+string(rune('1'+i)), d))
		}
		_, a := planning.Velocity(ws, "W", live, settings(), planning.UnitPoints, now)
		return a
	}
	cases := []struct {
		done []float64
		want float64
	}{
		{[]float64{1, 2, 3, 5}, 3.3}, // 実行中(完了 100)を含めず、直近 3 件(2・3・5)
		{[]float64{1, 2}, 1.5},
		{[]float64{4}, 4},
	}
	for _, c := range cases {
		got := avg(c.done...)
		if got == nil || math.Abs(*got-c.want) > 1e-9 {
			t.Fatalf("平均(%v) = %v, want %v", c.done, got, c.want)
		}
	}
	if got := avg(); got != nil {
		t.Fatalf("完了なしは nil: %v", *got)
	}
}

func TestVelocityOrder(t *testing.T) {
	localTokyo(t)
	planned := planning.Sprint{ID: "P", Name: "Plan", State: planning.Planned, Start: "2026-10-01", End: "2026-10-05"}
	deleted := closedWith("D", "Deleted", "2026-10-02", 1)
	deleted.Deleted = &change.Deletion{}
	live := []planning.Sprint{
		closedWith("L", "Late", "2026-10-20", 1), planned, closedWith("B", "Beta", "2026-10-10", 1),
		deleted, closedWith("A", "Alpha", "2026-10-10", 1), closedWith("E", "Early", "2026-10-05", 1),
	}
	bars, _ := planning.Velocity(nil, "W", live, settings(), planning.UnitPoints, jst(7, 12, 0))
	got := velocityIDs(bars)
	want := []change.ID{"E", "A", "B", "L"}
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
