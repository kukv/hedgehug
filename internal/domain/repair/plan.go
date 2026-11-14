package repair

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func always(change.Record) bool { return true }

// ExtraActive は、実行中のスプリントが 2 つ以上あるとき、計画中に戻すスプリント(内部 ID の順。決定 C5a-3)。
// 削除済みは数えない。残すのは、開始日、開始の行(state のいまの値の行)の at、その by(小文字)、行の ID の順で先頭の 1 つ。
func ExtraActive(wsLog []change.Record) []change.ID {
	type active struct {
		id    change.ID
		start string
		row   change.Record
	}
	var all []active
	for _, s := range planning.FromLog(wsLog).LiveSprints() {
		if s.State != planning.Active {
			continue
		}
		row, _ := change.Current(wsLog, change.Set, "sprints/"+string(s.ID)+"/state", always)
		all = append(all, active{s.ID, s.Start, row})
	}
	if len(all) < 2 {
		return nil
	}
	slices.SortFunc(all, func(a, b active) int {
		return cmp.Or(
			strings.Compare(a.start, b.start),
			a.row.At.Compare(b.row.At),
			strings.Compare(strings.ToLower(a.row.By), strings.ToLower(b.row.By)),
			strings.Compare(string(a.row.ID), string(b.row.ID)),
		)
	})
	var out []change.ID
	for _, a := range all[1:] {
		out = append(out, a.id)
	}
	slices.Sort(out)
	return out
}

// ItemLog は、アイテムと、そのアイテムのログ。
type ItemLog struct {
	Item item.Item
	Log  []change.Record
}

// sideBits は、rows のうち Facts にあるものの側を、自分の側を 1、remote の側を 2 の和で表す。
func sideBits(rows []change.Record, f Facts) int {
	bits := 0
	for _, r := range rows {
		bits |= f.Side(r.ID)
	}
	return bits
}

// crossed は、一方に自分の側の行、もう一方に remote の側の行があるか(どちらの向きでも)。
func crossed(a, b int) bool { return (a&1 != 0 && b&2 != 0) || (a&2 != 0 && b&1 != 0) }

// fieldRow は、アイテムのログの、欄 field のいまの値の行。欄の set の行がなければ作成の行(決定 C5a-4)。
func fieldRow(log []change.Record, field string) (change.Record, bool) {
	if r, ok := change.Current(log, change.Set, field, always); ok {
		return r, true
	}
	if c := change.Resolve(log, change.Options{}).Created; c != nil {
		return *c, true
	}
	return change.Record{}, false
}

// closingRows は、完了したスプリントごとの、完了の op の行の側(sideBits)。完了の記録のないものは含めない。
func closingRows(wsLog []change.Record, f Facts) map[change.ID]int {
	out := map[change.ID]int{}
	for _, s := range planning.FromLog(wsLog).Sprints {
		if s.State != planning.Closed || s.Closing == nil {
			continue
		}
		var rows []change.Record
		for _, r := range wsLog {
			if r.Op == s.Closing.Op {
				rows = append(rows, r)
			}
		}
		out[s.ID] = sideBits(rows, f)
	}
	return out
}

// ClosedSprintItems は、完了したスプリントに入った未完了の作業項目(内部 ID の順。決定 C5a-4)。
// 作業項目で削除済みでなく、ステータスの分類が未着手か進行中で、スプリントが完了のもののうち、
// 完了の op の行と、スプリントかステータスの欄のいまの値の行が、Facts の別々の側にあるもの。
// items はワークスペースのアイテムだけを渡す。
func ClosedSprintItems(wsLog []change.Record, st workspace.Settings, items []ItemLog, f Facts) []item.ID {
	closing := closingRows(wsLog, f)
	var out []item.ID
	for _, il := range items {
		it := il.Item
		cat := planning.Category(st, it.Status)
		if !planning.IsWork(it.Kind) || it.Deleted != nil || (cat != workspace.CategoryTodo && cat != workspace.CategoryDoing) {
			continue
		}
		bits, ok := closing[it.Sprint]
		if !ok {
			continue
		}
		var rows []change.Record
		for _, field := range []string{item.SprintField, "status"} {
			if r, ok := fieldRow(il.Log, field); ok {
				rows = append(rows, r)
			}
		}
		if crossed(bits, sideBits(rows, f)) {
			out = append(out, it.ID)
		}
	}
	slices.Sort(out)
	return out
}

// PlanRef は、存在しないスプリント・リリースを指すアイテムの欄。Field は item.SprintField か item.ReleaseField。
type PlanRef struct {
	Item   item.ID
	Field  string
	Target change.ID
}

// DanglingPlanRefs は、削除済みでないアイテムの sprint・release が、アイテムのワークスペースの計画
// (削除済みを含む)にないもの(決定 C5a-5)。アイテムの内部 ID の順で、sprint・release の順。
func DanglingPlanRefs(items []item.Item, plans map[workspace.ID]planning.Plan) []PlanRef {
	live := item.Live(items)
	slices.SortFunc(live, func(a, b item.Item) int { return strings.Compare(string(a.ID), string(b.ID)) })
	var out []PlanRef
	for _, it := range live {
		p := plans[it.Place.Workspace]
		if it.Sprint != "" && !slices.ContainsFunc(p.Sprints, func(s planning.Sprint) bool { return s.ID == it.Sprint }) {
			out = append(out, PlanRef{it.ID, item.SprintField, it.Sprint})
		}
		if it.Release != "" && !slices.ContainsFunc(p.Releases, func(r planning.Release) bool { return r.ID == it.Release }) {
			out = append(out, PlanRef{it.ID, item.ReleaseField, it.Release})
		}
	}
	return out
}
