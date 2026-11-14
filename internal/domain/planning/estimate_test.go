package planning_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// logged は、作業時間(100 分の 1 時間)hundredths の記録 n 件目を足したログ。
func logged(log []change.Record, n int, hundredths int) []change.Record {
	id := change.ID("L" + string(rune('0'+n)))
	return stampAt(log, jst(2, 10, n), change.NewWorklog(id, nil, worklog.Value(hundredths, "", "2026-11-02")))
}

func numbered(id string, number int, it item.Item) []change.Record {
	it.ID = item.ID(id)
	it.Place = item.Place{Workspace: "W", Number: number}
	it.Status = "Todo"
	return created(jst(1, 10, 0), it)
}

func hours(v float64) *float64 { return &v }

func intp(v int) *int { return &v }

// estimateWorks は、仕様の例: A・B・C が入り、D・E・Epic・削除済みは入らない。番号と逆の順に渡す。
func estimateWorks(t *testing.T) []planning.Work {
	t.Helper()
	a := logged(logged(numbered("A", 1, item.Item{Points: pt(3), Estimate: hours(4), Sprint: "S1"}), 1, 250), 2, 250)
	b := logged(numbered("B", 2, item.Item{Points: pt(0)}), 1, 150)
	c := logged(numbered("C", 3, item.Item{Estimate: hours(2), Sprint: "S2"}), 1, 175)
	d := logged(numbered("D", 4, item.Item{}), 1, 100)
	e := numbered("E", 5, item.Item{Points: pt(1), Estimate: hours(1)})
	epic := logged(numbered("P", 6, item.Item{Kind: item.Epic, Points: pt(1)}), 1, 100)
	del := stampAt(logged(numbered("X", 7, item.Item{Points: pt(1)}), 1, 100), jst(3, 10, 0), change.NewDeleteEntity(true))
	return works(t, del, epic, e, d, c, b, a)
}

func sameEstimate(t *testing.T, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %s, want %s", name, show(got), show(want))
	}
}

// show は、ポインタの中身を JSON で読める形にする(失敗の表示用)。
func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func TestEstimatesRows(t *testing.T) {
	rows, _ := planning.Estimates(estimateWorks(t), "W", "")

	if len(rows) != 3 {
		t.Fatalf("行数 = %d, want 3: %+v", len(rows), rows)
	}
	for i, id := range []item.ID{"A", "B", "C"} {
		if rows[i].Item.ID != id {
			t.Errorf("rows[%d] = %s, want %s", i, rows[i].Item.ID, id)
		}
	}
	a, b, c := rows[0], rows[1], rows[2]
	sameEstimate(t, "A.Estimate", a.Estimate, intp(400))
	sameEstimate(t, "A.Worked", a.Worked, 500)
	sameEstimate(t, "A.Diff", a.Diff, intp(100))
	sameEstimate(t, "A.PerPoint", a.PerPoint, pt(1.7))
	sameEstimate(t, "B.Estimate", b.Estimate, (*int)(nil))
	sameEstimate(t, "B.Worked", b.Worked, 150)
	sameEstimate(t, "B.Diff", b.Diff, (*int)(nil))
	sameEstimate(t, "B.PerPoint", b.PerPoint, (*float64)(nil))
	sameEstimate(t, "C.Points", c.Points, (*float64)(nil))
	sameEstimate(t, "C.Worked", c.Worked, 175)
	sameEstimate(t, "C.Diff", c.Diff, intp(-25))
	sameEstimate(t, "C.PerPoint", c.PerPoint, (*float64)(nil))
}

// 合計の差は、見積もり時間のある行(A と C)の差だけの合計。B の作業時間は差に入らない。
func TestEstimatesTotal(t *testing.T) {
	_, total := planning.Estimates(estimateWorks(t), "W", "")

	sameEstimate(t, "total", total, planning.EstimateTotal{Points: pt(3), Estimate: intp(600), Worked: 825, Diff: intp(75)})
}

// 見積もりのある行がなければ Estimate・Diff は nil、ポイントのある行がなければ Points は nil(F3-14)。
func TestEstimatesTotalNone(t *testing.T) {
	onlyPoints := logged(numbered("A", 1, item.Item{Points: pt(2)}), 1, 100)
	_, total := planning.Estimates(works(t, onlyPoints), "W", "")
	sameEstimate(t, "ポイントだけ", total, planning.EstimateTotal{Points: pt(2), Worked: 100})

	onlyEstimate := logged(numbered("A", 1, item.Item{Estimate: hours(1)}), 1, 100)
	_, total = planning.Estimates(works(t, onlyEstimate), "W", "")
	sameEstimate(t, "見積もりだけ", total, planning.EstimateTotal{Estimate: intp(100), Worked: 100, Diff: intp(0)})

	_, total = planning.Estimates(nil, "W", "")
	sameEstimate(t, "行なし", total, planning.EstimateTotal{})
}

func TestEstimatesSprint(t *testing.T) {
	rows, total := planning.Estimates(estimateWorks(t), "W", "S1")

	if len(rows) != 1 || rows[0].Item.ID != "A" {
		t.Fatalf("rows = %+v, want A だけ", rows)
	}
	sameEstimate(t, "Worked", total.Worked, 500)
}
