package planning

import (
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Layer は、累積フロー図の層(F3-12)。分類の層の Color は空(色は画面が分類で決める)。
type Layer struct {
	Key, Name, Color string
}

// FlowDay は、累積フロー図の 1 日。Counts は層と同じ順。
type FlowDay struct {
	Date   string
	Counts []int
}

// deletedLayer は、設定の一覧にないステータスにいた分の層。
var deletedLayer = Layer{Key: "deleted", Name: "(削除された値)"}

// categoryLayers は、分類の層。下から積む順。
var categoryLayers = []struct {
	Category workspace.Category
	Layer    Layer
}{
	{workspace.CategoryDone, Layer{Key: "done", Name: "完了"}},
	{workspace.CategoryDoing, Layer{Key: "doing", Name: "進行中"}},
	{workspace.CategoryTodo, Layer{Key: "todo", Name: "未着手"}},
	{workspace.CategoryCanceled, Layer{Key: "canceled", Name: "中止"}},
}

// Flow は、from〜to のすべての日の累積フロー図(05「累積フロー図」)。works は削除済みを含んでよい。
// 各日の値は、その日の終わり時点に作成済みの作業項目を、その日の終わり時点の層で分けた件数。層は下から上の順。
// ステータスの層には、一覧にないステータスにいた日が期間にあるときだけ、一番上に deletedLayer が付く。
func Flow(works []Work, ws workspace.ID, st workspace.Settings, from, to string, l Layers) ([]Layer, []FlowDay) {
	layers, slot := flowLayers(st, l)
	dates := flowDates(from, to)
	days := make([]FlowDay, len(dates))
	for i, d := range dates {
		days[i] = FlowDay{Date: d, Counts: make([]int, len(layers)+1)}
	}
	ends := make([]time.Time, len(dates))
	for i, d := range dates {
		ends[i] = EndOfDay(d, time.Local)
	}
	for _, w := range works {
		if !counted(w.Item, ws) {
			continue
		}
		countStatus(days, ends, steps(w.Log, "status"), slot)
	}
	return trimDeleted(layers, days)
}

// trimDeleted は、各日の Counts の最後(deletedLayer の分)を、期間に 1 件以上あるときだけ層に付け、なければ Counts から外す。
func trimDeleted(layers []Layer, days []FlowDay) ([]Layer, []FlowDay) {
	n := len(layers)
	for _, d := range days {
		if d.Counts[n] > 0 {
			return append(layers, deletedLayer), days
		}
	}
	for i := range days {
		days[i].Counts = days[i].Counts[:n]
	}
	return layers, days
}

// flowLayers は、層の分け方 l の層(下から上の順)と、ステータスの内部 ID から層の位置を引く関数。
// 一覧にないステータスの位置は、分類なら未着手、ステータスなら deletedLayer(層の数)。
func flowLayers(st workspace.Settings, l Layers) ([]Layer, func(change.ID) int) {
	if l == LayersStatus {
		return statusLayers(st)
	}
	var layers []Layer
	for _, c := range categoryLayers {
		layers = append(layers, c.Layer)
	}
	return layers, func(id change.ID) int {
		cat := Category(st, id)
		for i, c := range categoryLayers {
			if c.Category == cat {
				return i
			}
		}
		return 0
	}
}

// statusLayers は、ステータスの層。分類の順にまとめ、分類の中は定義順。
func statusLayers(st workspace.Settings) ([]Layer, func(change.ID) int) {
	var layers []Layer
	pos := map[change.ID]int{}
	for _, c := range categoryLayers {
		for _, v := range st.Statuses {
			if v.Category == c.Category {
				pos[v.ID] = len(layers)
				layers = append(layers, Layer{Key: string(v.ID), Name: v.Name, Color: string(v.Color)})
			}
		}
	}
	n := len(layers)
	return layers, func(id change.ID) int {
		if i, ok := pos[id]; ok {
			return i
		}
		return n
	}
}

// flowDates は、from〜to のすべての日(YYYY-MM-DD)。読めない日付は空。
func flowDates(from, to string) []string {
	d, err := time.Parse(time.DateOnly, from)
	if err != nil {
		return nil
	}
	var out []string
	for ymd := d.Format(time.DateOnly); ymd <= to; ymd = d.Format(time.DateOnly) {
		out = append(out, ymd)
		d = d.AddDate(0, 0, 1)
	}
	return out
}

// countStatus は、1 件のステータスの変遷 ss を、各日(終わりは ends)の層の件数 days に足す(F3-13)。
// 日は進む順なので、点の位置は戻らない。作成前の日は足さない。
func countStatus(days []FlowDay, ends []time.Time, ss []step, slot func(change.ID) int) {
	i := 0
	for d, end := range ends {
		if len(ss) == 0 || !ss[0].At.Before(end) {
			continue
		}
		for i+1 < len(ss) && ss[i+1].At.Before(end) {
			i++
		}
		days[d].Counts[slot(ss[i].Value)]++
	}
}
