package planning

import (
	"encoding/json"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Work は、集計に使うアイテムと、そのログ(ある時点の値を求めるのに使う)。
type Work struct {
	Item item.Item
	Log  []change.Record
}

// IsWork は、作業項目(Story・Task・Bug)の種類か(05「数え方の共通ルール」)。
func IsWork(k item.Kind) bool { return k == item.Story || k == item.Task || k == item.Bug }

// Category は、ステータス status の分類。設定にないステータスと空は未着手。
func Category(st workspace.Settings, status change.ID) workspace.Category {
	for _, v := range st.Statuses {
		if v.ID == status {
			return v.Category
		}
	}
	return workspace.CategoryTodo
}

// EndOfDay は、日付 ymd の終わり時点(loc の翌日の 0 時。F1-9)。読めない日付は時刻ゼロ。
func EndOfDay(ymd string, loc *time.Location) time.Time {
	d, err := time.ParseInLocation(time.DateOnly, ymd, loc)
	if err != nil {
		return time.Time{}
	}
	return d.AddDate(0, 0, 1)
}

// ValueAt は、時点 t の項目 f の値。at が t より前の行だけで change.Resolve する(作成の行の値を含む)。
// その行に作成がなければ false。作成はあるが f の値がなければ nil と true。
func ValueAt(log []change.Record, f string, t time.Time) (json.RawMessage, bool) {
	var rows []change.Record
	for _, r := range log {
		if r.At.Before(t) {
			rows = append(rows, r)
		}
	}
	s := change.Resolve(rows, change.Options{})
	if s.Created == nil {
		return nil, false
	}
	return s.Values[f], true
}

// Amount は、アイテムの量(ポイント。未設定は 0)。
func Amount(it item.Item) float64 {
	if it.Points == nil {
		return 0
	}
	return *it.Points
}

// Confirm は、スプリント s の完了の確定値(F1-10)。works は削除済みを含んでよい。
// 計画は、開始日(今日より後なら今日)の終わり時点でスプリントに入っていて、その時点の分類が中止でないもの。
// 実績は、いまスプリントに入っていて分類が完了のもの。量はどちらもいまのポイント。
func Confirm(works []Work, ws workspace.ID, s Sprint, st workspace.Settings, now time.Time) Confirmed {
	day := min(s.Start, Today(now))
	t := EndOfDay(day, now.Location())
	var c Confirmed
	for _, w := range works {
		if !counted(w.Item, ws) {
			continue
		}
		if idAt(w.Log, item.SprintField, t) == s.ID && Category(st, idAt(w.Log, "status", t)) != workspace.CategoryCanceled {
			c.Committed.add(w.Item)
		}
		if w.Item.Sprint == s.ID && Category(st, w.Item.Status) == workspace.CategoryDone {
			c.Completed.add(w.Item)
		}
	}
	return c
}

// counted は、ワークスペース ws で数える作業項目か(削除済みと、いま別のワークスペースにあるものを除く)。
func counted(it item.Item, ws workspace.ID) bool {
	return it.Deleted == nil && it.Place.Workspace == ws && IsWork(it.Kind)
}

func (t *Totals) add(it item.Item) {
	t.Points += Amount(it)
	t.Count++
}

// idAt は、時点 t の項目 f の内部 ID の値。作成前・値なし・null は空。
func idAt(log []change.Record, f string, t time.Time) change.ID {
	raw, ok := ValueAt(log, f, t)
	var v change.ID
	if !ok || json.Unmarshal(raw, &v) != nil {
		return ""
	}
	return v
}
