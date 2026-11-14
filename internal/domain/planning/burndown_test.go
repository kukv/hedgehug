package planning_test

import (
	"math"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// burnSprint は、開始 2026-11-02、終了 2026-11-05(4 日)の実行中のスプリント S1。
func burnSprint() planning.Sprint {
	return planning.Sprint{ID: "S1", Name: "Sprint 1", State: planning.Active, Start: "2026-11-02", End: "2026-11-05"}
}

// closedSprint は、11/5 10:00 に op close1 で完了したスプリント S1。確定値は works から Confirm で求める。
func closedSprint(t *testing.T, ws []planning.Work) planning.Sprint {
	t.Helper()
	s := burnSprint()
	s.State = planning.Closed
	s.Closing = &planning.Closing{At: jst(5, 10, 0), Op: "close1"}
	c := planning.Confirm(ws, "W", s, settings(), jst(5, 10, 0))
	s.Confirmed = &c
	return s
}

// inS1 は、11/1 に S1 に入れて作った、ステータス Todo のアイテムのログ。
func inS1(id string, points *float64) []change.Record {
	return created(jst(1, 10, 0), item.Item{ID: change.ID(id), Sprint: "S1", Status: "Todo", Points: points})
}

// closeRow は、op の行として drafts を書く(完了の行と同じ op の set sprint を作る)。
func closeRow(log []change.Record, when time.Time, op change.ID, ds ...change.Draft) []change.Record {
	for i := range ds {
		ds[i] = ds[i].InOp(op)
	}
	return stampAt(log, when, ds...)
}

func works(t *testing.T, logs ...[]change.Record) []planning.Work {
	t.Helper()
	var out []planning.Work
	for _, l := range logs {
		out = append(out, work(t, l))
	}
	return out
}

// remainings は、各日の残り。nil は -1。
func remainings(b planning.Burndown) []float64 {
	out := make([]float64, len(b.Days))
	for i, d := range b.Days {
		out[i] = -1
		if d.Remaining != nil {
			out[i] = *d.Remaining
		}
	}
	return out
}

func totals(b planning.Burndown) []float64 {
	out := make([]float64, len(b.Days))
	for i, d := range b.Days {
		out[i] = d.Total
	}
	return out
}

func ideals(b planning.Burndown) []float64 {
	out := make([]float64, len(b.Days))
	for i, d := range b.Days {
		out[i] = d.Ideal
	}
	return out
}

func sameFloats(t *testing.T, name string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
}

func TestBurnDaysAndIdeal(t *testing.T) {
	localTokyo(t)
	a := inS1("A", pt(3))
	b := setStatus(inS1("B", pt(5)), jst(4, 10, 0), "Todo", "Done")
	got := planning.Burn(works(t, a, b), "W", burnSprint(), settings(), planning.UnitPoints, jst(4, 12, 0))
	if len(got.Days) != 4 || got.Days[0].Date != "2026-11-02" || got.Days[3].Date != "2026-11-05" || got.Shown != 3 {
		t.Fatalf("Days = %+v, Shown = %d", got.Days, got.Shown)
	}
	sameFloats(t, "Total", totals(got), []float64{8, 8, 8, 8})
	sameFloats(t, "Remaining", remainings(got), []float64{8, 8, 3, -1})
	sameFloats(t, "Ideal", ideals(got), []float64{8, 8 - 8.0/3, 8 - 16.0/3, 0})
	if got.Committed != nil {
		t.Fatalf("実行中は Committed なし: %v", *got.Committed)
	}
}

func TestBurnScopeChange(t *testing.T) {
	localTokyo(t)
	a := setStatus(inS1("A", pt(3)), jst(4, 10, 0), "Todo", "Canceled")
	b := inS1("B", pt(5))
	c := setSprint(created(jst(1, 10, 0), item.Item{ID: "C", Status: "Todo", Points: pt(2)}), jst(3, 10, 0), nil, "S1")
	d := setSprint(inS1("D", pt(4)), jst(3, 10, 0), "S1", nil)
	got := planning.Burn(works(t, a, b, c, d), "W", burnSprint(), settings(), planning.UnitPoints, jst(4, 12, 0))
	sameFloats(t, "Total", totals(got), []float64{12, 10, 7, 7})
	sameFloats(t, "Ideal(開始日の総量 12 から引き直さない)", ideals(got), []float64{12, 8, 4, 0})
	sameFloats(t, "Remaining", remainings(got), []float64{12, 10, 7, -1})
}

func TestBurnCarryOver(t *testing.T) {
	localTokyo(t)
	s := burnSprint()
	s.State = planning.Closed
	s.Closing = &planning.Closing{At: jst(4, 15, 0), Op: "close1"}
	carry := func(id string, p float64) []change.Record {
		return closeRow(inS1(id, pt(p)), jst(4, 15, 0), "close1", change.NewSet(item.SprintField, "S1", "S2"))
	}
	a := setSprint(carry("A", 3), jst(4, 16, 0), "S2", nil) // 同じ日にさらにバックログへ移した
	c := carry("C", 2)
	b := setSprint(inS1("B", pt(7)), jst(4, 14, 0), "S1", nil) // 完了の前に外した
	e := setStatus(inS1("E", pt(1)), jst(3, 10, 0), "Todo", "Done")
	got := planning.Burn(works(t, a, b, c, e), "W", s, settings(), planning.UnitPoints, jst(6, 12, 0))
	if got.Shown != 3 {
		t.Fatalf("Shown = %d, want 3(完了した日まで)", got.Shown)
	}
	r := remainings(got)
	if r[2] != 5 {
		t.Fatalf("11/04 の残り = %v, want 5(完了で移した A・C は完了した日の終わりまで範囲に残る)", r)
	}
	if got.Days[2].Total != 6 {
		t.Fatalf("11/04 の総量 = %v, want 6(B は範囲に入らない)", got.Days[2].Total)
	}
	if r[3] != -1 {
		t.Fatalf("完了した日より後は残りなし: %v", r)
	}
}

func TestBurnCommittedNote(t *testing.T) {
	localTokyo(t)
	now := jst(6, 12, 0)
	a, b := inS1("A", pt(3)), inS1("B", pt(5))

	plain := works(t, a, b)
	if got := planning.Burn(plain, "W", closedSprint(t, plain), settings(), planning.UnitPoints, now); got.Committed != nil {
		t.Fatalf("何も変えていない: %v", *got.Committed)
	}

	added := setSprint(created(jst(1, 10, 0), item.Item{ID: "C", Status: "Todo", Points: pt(2)}), jst(3, 10, 0), nil, "S1")
	withAdded := works(t, a, b, added)
	if got := planning.Burn(withAdded, "W", closedSprint(t, withAdded), settings(), planning.UnitPoints, now); got.Committed != nil {
		t.Fatalf("途中の追加は開始日の総量にも確定の計画にも入らない: %v", *got.Committed)
	}

	s := closedSprint(t, plain)
	later := works(t, a, stampAt(b, jst(6, 9, 0), change.NewDeleteEntity(true)))
	got := planning.Burn(later, "W", s, settings(), planning.UnitPoints, now)
	if got.Committed == nil || *got.Committed != 8 {
		t.Fatalf("完了のあとの削除: Committed = %v, want 8", got.Committed)
	}
}

func TestBurnCommittedNoteCount(t *testing.T) {
	localTokyo(t)
	a, b := inS1("A", pt(3)), inS1("B", nil)
	s := closedSprint(t, works(t, a, b))
	later := works(t, a, stampAt(b, jst(6, 9, 0), change.NewDeleteEntity(true)))
	if got := planning.Burn(later, "W", s, settings(), planning.UnitPoints, jst(6, 12, 0)); got.Committed != nil {
		t.Fatalf("ポイントは 3 のままで同じ: %v", *got.Committed)
	}
	got := planning.Burn(later, "W", s, settings(), planning.UnitCount, jst(6, 12, 0))
	if got.Committed == nil || *got.Committed != 2 {
		t.Fatalf("件数の確定値 2 と比べる: %v", got.Committed)
	}
}

func TestBurnCommittedFloatNoise(t *testing.T) {
	localTokyo(t)
	// 0.1 + 0.2 と 0.3 は浮動小数点では違うが、画面の桁では同じなので注記を出さない。
	ws := works(t, inS1("A", pt(0.1)), inS1("B", pt(0.2)))
	s := closedSprint(t, ws)
	s.Confirmed.Committed.Points = 0.3
	if got := planning.Burn(ws, "W", s, settings(), planning.UnitPoints, jst(6, 12, 0)); got.Committed != nil {
		t.Fatalf("12 桁で同じ: %v", *got.Committed)
	}
}

func TestBurnFutureStart(t *testing.T) {
	localTokyo(t)
	got := planning.Burn(works(t, inS1("A", pt(3))), "W", burnSprint(), settings(), planning.UnitPoints, jst(1, 12, 0))
	if got.Shown != 0 || len(got.Days) != 4 {
		t.Fatalf("Shown = %d, Days = %d", got.Shown, len(got.Days))
	}
	for _, d := range got.Days {
		if d.Remaining != nil {
			t.Fatalf("残りはすべて nil: %+v", d)
		}
	}
	// F3-9: 総量は表示する日より後も、その日の終わり時点で求めた値を出す。
	sameFloats(t, "Total", totals(got), []float64{3, 3, 3, 3})
}

func TestBurnUnitCount(t *testing.T) {
	localTokyo(t)
	got := planning.Burn(works(t, inS1("A", pt(3)), inS1("B", nil)), "W", burnSprint(), settings(), planning.UnitCount, jst(2, 12, 0))
	sameFloats(t, "Total", totals(got), []float64{2, 2, 2, 2})
	sameFloats(t, "Remaining", remainings(got), []float64{2, -1, -1, -1})
}

func TestBurnExcluded(t *testing.T) {
	localTokyo(t)
	deleted := stampAt(inS1("D", pt(1)), jst(4, 10, 0), change.NewDeleteEntity(true))
	other := created(jst(1, 10, 0), item.Item{ID: "O", Place: item.Place{Workspace: "W2"}, Sprint: "S1", Status: "Todo", Points: pt(2)})
	epic := created(jst(1, 10, 0), item.Item{ID: "P", Kind: item.Epic, Sprint: "S1", Status: "Todo", Points: pt(4)})
	atCreate := created(jst(3, 10, 0), item.Item{ID: "N", Sprint: "S1", Status: "Todo", Points: pt(8)})
	gone := setSprint(inS1("G", pt(16)), jst(3, 10, 0), "S1", nil) // スプリントの削除で外れた
	got := planning.Burn(works(t, deleted, other, epic, atCreate, gone, inS1("A", pt(32))), "W", burnSprint(), settings(), planning.UnitPoints, jst(4, 12, 0))
	// D は削除済み、O は別のワークスペース、P は Epic なので、過去の日も数えない。
	// N は作成の日(11/3)から、G は 11/3 の終わりまでは外れているので 11/2 だけ。
	sameFloats(t, "Total", totals(got), []float64{16 + 32, 8 + 32, 8 + 32, 8 + 32})
}

func TestAdded(t *testing.T) {
	localTokyo(t)
	a := inS1("A", pt(3))
	moved := setSprint(inS1("B", pt(5)), jst(4, 9, 0), "S1", nil) // 開始日にあって今日外れた
	c := setSprint(created(jst(1, 10, 0), item.Item{ID: "C", Status: "Todo", Points: pt(2)}), jst(4, 9, 0), nil, "S1")
	d := setStatus(setSprint(created(jst(1, 10, 0), item.Item{ID: "D", Status: "Todo", Points: pt(1)}), jst(4, 9, 0), nil, "S1"), jst(4, 10, 0), "Todo", "Canceled")
	ws := works(t, a, moved, c, d)
	if got := planning.Added(ws, "W", burnSprint(), settings(), planning.UnitPoints, jst(4, 12, 0)); got != 2 {
		t.Fatalf("Added = %v, want 2", got)
	}
	if got := planning.Added(ws, "W", burnSprint(), settings(), planning.UnitCount, jst(4, 12, 0)); got != 1 {
		t.Fatalf("件数 Added = %v, want 1", got)
	}
	if got := planning.Added(ws, "W", burnSprint(), settings(), planning.UnitPoints, jst(1, 12, 0)); got != 0 {
		t.Fatalf("今日が開始日より前なら 0: %v", got)
	}
}

func TestProgress(t *testing.T) {
	mk := func(id string, kind item.Kind, status string, p float64) item.Item {
		return item.Item{ID: change.ID(id), Kind: kind, Status: change.ID(status), Sprint: "S1", Points: pt(p), Place: item.Place{Workspace: "W"}}
	}
	del := mk("X", item.Task, "Done", 16)
	del.Deleted = &change.Deletion{}
	its := []item.Item{
		mk("A", item.Story, "Done", 3), mk("B", item.Bug, "Doing", 5), mk("C", item.Task, "Canceled", 8),
		mk("E", item.Epic, "Done", 32), del,
	}
	done, total := planning.Progress(its, "W", "S1", settings())
	if want := (planning.Totals{Points: 3, Count: 1}); done != want {
		t.Fatalf("done = %+v, want %+v", done, want)
	}
	if want := (planning.Totals{Points: 8, Count: 2}); total != want {
		t.Fatalf("total = %+v, want %+v", total, want)
	}
}
