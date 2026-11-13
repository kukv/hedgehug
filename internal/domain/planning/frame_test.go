package planning_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

func TestFrameEnd(t *testing.T) {
	day1 := jst(1, 10, 0)
	deleted := sprint1()
	deleted.ID = "S9"
	deleted.Deleted = &change.Deletion{Op: "x", At: day1, Root: true}
	plan := planning.Plan{Sprints: []planning.Sprint{sprint1(), deleted}}
	logs := [][]change.Record{
		created(day1, item.Item{ID: "A", Sprint: "S1", Status: "Todo", Rank: "c"}),
		created(day1, item.Item{ID: "B", Sprint: "S1", Status: "Canceled", Rank: "f"}), // スプリントの枠は分類を問わない
		created(day1, item.Item{ID: "C", Kind: item.Epic, Sprint: "S1", Status: "Todo", Rank: "z"}),
		created(day1, item.Item{ID: "D", Place: item.Place{Workspace: "W2"}, Sprint: "S1", Rank: "y"}),
		stampAt(created(day1, item.Item{ID: "E", Sprint: "S1", Rank: "x"}), jst(2, 10, 0), change.NewDeleteEntity(true)),
		created(day1, item.Item{ID: "F", Status: "Todo", Rank: "b"}),
		created(day1, item.Item{ID: "G", Sprint: "S9", Status: "Doing", Rank: "d"}),  // 削除済みのスプリントはバックログ
		created(day1, item.Item{ID: "H", Sprint: "Gone", Status: "Gone", Rank: "e"}), // ないスプリント・設定にないステータス
		created(day1, item.Item{ID: "I", Status: "Done", Rank: "w"}),
		created(day1, item.Item{ID: "J", Status: "Canceled", Rank: "v"}),
		created(day1, item.Item{ID: "K", Kind: item.Epic, Status: "Todo", Rank: "u"}),
	}
	var works []planning.Work
	for _, log := range logs {
		works = append(works, work(t, log))
	}
	st := settings()
	if got := planning.FrameEnd(works, "W", "S1", plan, st); got != "f" {
		t.Fatalf("S1 の枠 = %q, want f", got)
	}
	if got := planning.FrameEnd(works, "W", "", plan, st); got != "e" {
		t.Fatalf("バックログの枠 = %q, want e", got)
	}
	if got := planning.FrameEnd(works, "W", "S2", plan, st); got != "" {
		t.Fatalf("空の枠 = %q, want 空", got)
	}
	if got := planning.FrameEnd(nil, "W", "", plan, st); got != "" {
		t.Fatalf("空のバックログ = %q, want 空", got)
	}
}
