package planning_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// flowSettings は、ワークスペースの既定の設定(ステータスの ID は S1〜S5 の順)。
func flowSettings() workspace.Settings {
	n := 0
	return workspace.DefaultSettings(func() change.ID {
		n++
		return change.ID("S" + strconv.Itoa(n))
	})
}

// statusID は、既定の設定のステータス名 name の内部 ID。
func statusID(t *testing.T, st workspace.Settings, name string) change.ID {
	t.Helper()
	for _, v := range st.Statuses {
		if v.Name == name {
			return v.ID
		}
	}
	t.Fatalf("ステータス %q がない", name)
	return ""
}

func flowCounts(days []planning.FlowDay) [][]int {
	out := make([][]int, len(days))
	for i, d := range days {
		out[i] = d.Counts
	}
	return out
}

func layerKeys(ls []planning.Layer) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Key
	}
	return out
}

func layerNames(ls []planning.Layer) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Name
	}
	return out
}

func flowDates(days []planning.FlowDay) []string {
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = d.Date
	}
	return out
}

func sameFlow[T any](t *testing.T, name string, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestFlowCategory(t *testing.T) {
	localTokyo(t)
	st := flowSettings()
	todo, doing, done, cancel := statusID(t, st, "Todo"), statusID(t, st, "InProgress"), statusID(t, st, "Done"), statusID(t, st, "Cancel")
	a := created(jst(1, 10, 0), item.Item{ID: "A", Status: todo})
	a = setStatus(a, jst(2, 10, 0), todo, doing)
	a = setStatus(a, jst(3, 10, 0), doing, done)
	b := created(jst(2, 10, 0), item.Item{ID: "B", Status: todo})
	b = setStatus(b, jst(3, 10, 0), todo, cancel)

	layers, days := planning.Flow(works(t, a, b), "W", st, "2026-11-01", "2026-11-03", planning.LayersCategory)

	sameFlow(t, "keys", layerKeys(layers), []string{"done", "doing", "todo", "canceled"})
	sameFlow(t, "names", layerNames(layers), []string{"完了", "進行中", "未着手", "中止"})
	for _, l := range layers {
		if l.Color != "" {
			t.Errorf("分類の層の色は空: %+v", l)
		}
	}
	sameFlow(t, "dates", flowDates(days), []string{"2026-11-01", "2026-11-02", "2026-11-03"})
	sameFlow(t, "counts", flowCounts(days), [][]int{{0, 0, 1, 0}, {0, 1, 1, 0}, {1, 0, 0, 1}})
}

func TestFlowStatus(t *testing.T) {
	localTokyo(t)
	st := flowSettings()
	todo, doing, done := statusID(t, st, "Todo"), statusID(t, st, "InProgress"), statusID(t, st, "Done")
	pending, cancel := statusID(t, st, "Pending"), statusID(t, st, "Cancel")
	a := created(jst(1, 10, 0), item.Item{ID: "A", Status: todo})
	a = setStatus(a, jst(2, 10, 0), todo, doing)
	a = setStatus(a, jst(3, 10, 0), doing, done)
	c := created(jst(1, 10, 0), item.Item{ID: "C", Status: pending})

	layers, days := planning.Flow(works(t, a, c), "W", st, "2026-11-01", "2026-11-03", planning.LayersStatus)

	sameFlow(t, "keys", layerKeys(layers), []string{string(done), string(doing), string(todo), string(pending), string(cancel)})
	sameFlow(t, "names", layerNames(layers), []string{"Done", "InProgress", "Todo", "Pending", "Cancel"})
	if layers[0].Color != "green" || layers[3].Color != "yellow" {
		t.Errorf("色はステータスのもの: %+v", layers)
	}
	sameFlow(t, "counts", flowCounts(days), [][]int{{0, 0, 1, 1, 0}, {0, 1, 0, 1, 0}, {1, 0, 0, 1, 0}})

	// 一覧にないステータスにいた日があれば、一番上に (削除された値) の層が付く。
	gone := created(jst(1, 10, 0), item.Item{ID: "G", Status: "Gone"})
	gone = setStatus(gone, jst(2, 10, 0), "Gone", todo)
	layers, days = planning.Flow(works(t, a, gone), "W", st, "2026-11-01", "2026-11-03", planning.LayersStatus)
	last := layers[len(layers)-1]
	if len(layers) != 6 || last.Key != "deleted" || last.Name != "(削除された値)" {
		t.Fatalf("deleted の層が最後に付く: %+v", layers)
	}
	sameFlow(t, "counts", flowCounts(days), [][]int{{0, 0, 1, 0, 0, 1}, {0, 1, 1, 0, 0, 0}, {1, 0, 1, 0, 0, 0}})

	// 期間のどの日にも一覧にないステータスがなければ、層は付かない(期間を 11/02 からにする)。
	layers, days = planning.Flow(works(t, a, gone), "W", st, "2026-11-02", "2026-11-03", planning.LayersStatus)
	if len(layers) != 5 || len(days[0].Counts) != 5 {
		t.Errorf("deleted の層は付かない: %+v %v", layerKeys(layers), flowCounts(days))
	}

	// 分類の層では、一覧にないステータスは未着手に数える。
	_, days = planning.Flow(works(t, gone), "W", st, "2026-11-01", "2026-11-01", planning.LayersCategory)
	sameFlow(t, "category counts", flowCounts(days), [][]int{{0, 0, 1, 0}})
}

func TestFlowExcluded(t *testing.T) {
	localTokyo(t)
	st := flowSettings()
	todo, cancel := statusID(t, st, "Todo"), statusID(t, st, "Cancel")
	live := created(jst(2, 10, 0), item.Item{ID: "L", Status: todo})
	canceled := created(jst(2, 11, 0), item.Item{ID: "X", Status: cancel})
	removed := work(t, created(jst(1, 10, 0), item.Item{ID: "D", Status: todo}))
	removed.Item.Deleted = &change.Deletion{}
	other := created(jst(1, 10, 0), item.Item{ID: "O", Status: todo, Place: item.Place{Workspace: "Z"}})
	epic := created(jst(1, 10, 0), item.Item{ID: "E", Status: todo, Kind: item.Epic})
	step := created(jst(1, 10, 0), item.Item{ID: "P", Status: todo, Kind: item.Step})
	ws := append(works(t, live, canceled, other, epic, step), removed)

	_, days := planning.Flow(ws, "W", st, "2026-11-01", "2026-11-02", planning.LayersCategory)

	// 11/01 は L・X の作成前で、ほかは数えない項目。11/02 は L が未着手、X が中止。
	sameFlow(t, "counts", flowCounts(days), [][]int{{0, 0, 0, 0}, {0, 0, 1, 1}})
}
