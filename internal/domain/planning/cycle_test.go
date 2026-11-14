package planning_test

import (
	"math"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// cycleSettings は、settings に完了の分類のもう 1 つのステータス Released を足したもの。
func cycleSettings() workspace.Settings {
	st := settings()
	st.Statuses = append(st.Statuses, workspace.Value{ID: "Released", Category: workspace.CategoryDone})
	return st
}

// wantDays は、サイクルタイム・リードタイムの件数と中央値を確かめる(中央値が nil なら want は nil)。
func wantDays(t *testing.T, name string, got planning.Durations, n int, median *float64) {
	t.Helper()
	if got.N != n {
		t.Errorf("%s N = %d, want %d", name, got.N, n)
	}
	switch {
	case median == nil && got.Median != nil:
		t.Errorf("%s Median = %v, want nil", name, *got.Median)
	case median != nil && got.Median == nil:
		t.Errorf("%s Median = nil, want %v", name, *median)
	case median != nil && math.Abs(*got.Median-*median) > 1e-9:
		t.Errorf("%s Median = %v, want %v", name, *got.Median, *median)
	}
}

func cycleOf(t *testing.T, logs ...[]change.Record) (cycle, lead planning.Durations) {
	t.Helper()
	return planning.Cycle(works(t, logs...), "W", cycleSettings())
}

func TestCycleBasic(t *testing.T) {
	localTokyo(t)
	log := created(jst(1, 0, 0), item.Item{ID: "A", Status: "Todo"})
	log = setStatus(log, jst(2, 0, 0), "Todo", "Doing")
	log = setStatus(log, jst(4, 12, 0), "Doing", "Done")

	cycle, lead := cycleOf(t, log)

	wantDays(t, "cycle", cycle, 1, pt(2.5))
	wantDays(t, "lead", lead, 1, pt(3.5))
	if cycle.Skipped != 0 || lead.Skipped != 0 {
		t.Errorf("Skipped = %d, %d", cycle.Skipped, lead.Skipped)
	}
}

// 再び開いたものは、再び開いた時刻が着手、作成からがリードタイム。Released への移動は到達に数えない。
func TestCycleReopened(t *testing.T) {
	localTokyo(t)
	log := created(jst(1, 0, 0), item.Item{ID: "A", Status: "Todo"})
	log = setStatus(log, jst(2, 0, 0), "Todo", "Doing")
	log = setStatus(log, jst(3, 0, 0), "Doing", "Done")
	log = setStatus(log, jst(5, 0, 0), "Done", "Doing")
	log = setStatus(log, jst(6, 0, 0), "Doing", "Done")

	cycle, lead := cycleOf(t, log)
	wantDays(t, "cycle", cycle, 1, pt(1))
	wantDays(t, "lead", lead, 1, pt(5))

	log = setStatus(log, jst(8, 0, 0), "Done", "Released")
	cycle, lead = cycleOf(t, log)
	wantDays(t, "released cycle", cycle, 1, pt(1))
	wantDays(t, "released lead", lead, 1, pt(5))
}

func TestCycleNoStart(t *testing.T) {
	localTokyo(t)
	direct := created(jst(1, 0, 0), item.Item{ID: "A", Status: "Todo"})
	direct = setStatus(direct, jst(3, 0, 0), "Todo", "Done")

	cycle, lead := cycleOf(t, direct)
	wantDays(t, "cycle", cycle, 0, nil)
	wantDays(t, "lead", lead, 1, pt(2))
	if cycle.Skipped != 1 || lead.Skipped != 0 {
		t.Errorf("Skipped = %d, %d", cycle.Skipped, lead.Skipped)
	}

	// 作成のときから完了、いまの分類が完了でない、中止は、どちらにも数えない。
	born := created(jst(1, 0, 0), item.Item{ID: "B", Status: "Done"})
	open := created(jst(1, 0, 0), item.Item{ID: "C", Status: "Todo"})
	open = setStatus(open, jst(2, 0, 0), "Todo", "Doing")
	reopened := created(jst(1, 0, 0), item.Item{ID: "D", Status: "Todo"})
	reopened = setStatus(reopened, jst(2, 0, 0), "Todo", "Done")
	reopened = setStatus(reopened, jst(3, 0, 0), "Done", "Doing")
	canceled := created(jst(1, 0, 0), item.Item{ID: "E", Status: "Todo"})
	canceled = setStatus(canceled, jst(2, 0, 0), "Todo", "Canceled")
	cycle, lead = cycleOf(t, born, open, reopened, canceled)
	wantDays(t, "cycle", cycle, 0, nil)
	wantDays(t, "lead", lead, 0, nil)
	if cycle.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", cycle.Skipped)
	}

	// 作成のときから進行中のものは、着手が作成の時刻。
	doing := created(jst(1, 0, 0), item.Item{ID: "F", Status: "Doing"})
	doing = setStatus(doing, jst(3, 0, 0), "Doing", "Done")
	cycle, lead = cycleOf(t, doing)
	wantDays(t, "cycle", cycle, 1, pt(2))
	wantDays(t, "lead", lead, 1, pt(2))
}

// 時計のずれで、完了の行の at が作成以前でも、0 日として数える(prev が作成の行を指す)。
func TestCycleNegative(t *testing.T) {
	localTokyo(t)
	for name, doneAt := range map[string]time.Time{"before": jst(1, 8, 0), "equal": jst(1, 9, 0)} {
		log := created(jst(1, 9, 0), item.Item{ID: "A", Status: "Doing"})
		log = setStatus(log, doneAt, "Doing", "Done")

		cycle, lead := cycleOf(t, log)

		wantDays(t, name+" cycle", cycle, 1, pt(0))
		wantDays(t, name+" lead", lead, 1, pt(0))
	}
}

// doneIn は、11/1 0:00 に進行中で作り、days 日後に完了にしたアイテムのログ。
func doneIn(id string, days int) []change.Record {
	log := created(jst(1, 0, 0), item.Item{ID: change.ID(id), Status: "Doing"})
	return setStatus(log, jst(1, 0, 0).AddDate(0, 0, days), "Doing", "Done")
}

func TestCycleStats(t *testing.T) {
	localTokyo(t)
	cycle, _ := cycleOf(t, doneIn("A", 1), doneIn("B", 2), doneIn("C", 3), doneIn("D", 4), doneIn("E", 10))
	wantDays(t, "five", cycle, 5, pt(3))
	if cycle.P85 == nil || *cycle.P85 != 6.4 {
		t.Errorf("P85 = %v, want 6.4", cycle.P85)
	}

	cycle, _ = cycleOf(t, doneIn("A", 1), doneIn("B", 2), doneIn("C", 3), doneIn("D", 4))
	wantDays(t, "four", cycle, 4, pt(2.5))
	if cycle.P85 == nil || *cycle.P85 != 3.6 {
		t.Errorf("P85 = %v, want 3.6", cycle.P85)
	}

	cycle, lead := cycleOf(t)
	wantDays(t, "empty cycle", cycle, 0, nil)
	wantDays(t, "empty lead", lead, 0, nil)
	if cycle.P85 != nil || lead.P85 != nil {
		t.Errorf("P85 は nil: %v %v", cycle.P85, lead.P85)
	}
}
