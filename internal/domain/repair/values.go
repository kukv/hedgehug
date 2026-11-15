package repair

import (
	"slices"
	"sort"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// RestoreReason は、値の削除を取り消す理由(決定 C5b2-4・C5b2-9)。指している項目(initial_status・
// category_default/<分類>・initial_priority)の Field か、0 個になる分類の Category の、どちらか一方だけを持つ。
type RestoreReason struct {
	Field    string
	Category workspace.Category
}

// pointer は、設定の項目 Field が指す、削除された値の f(statuses/<ID>・priorities/<ID>)。
type pointer struct {
	Field, F string
}

// valueView は、ワークスペースのログの、削除された値の見え方。
type valueView struct {
	// deleted は、値の f → その値を覆う、取り消されていない削除の行(ログの順)。
	deleted map[string][]change.Record
	// all は、削除済みのステータスを含めた設定。
	all workspace.Settings
	// pointers は、削除された値を指す項目(初期ステータス・分類の既定(分類の順)・初期優先度の順)。
	pointers []pointer
}

// readValues は、log の削除された値を読む。ワークスペースの作成の行がなければ false。
func readValues(log []change.Record) (valueView, bool) {
	live, ok := workspace.SettingsFromLog(log)
	if !ok {
		return valueView{}, false
	}
	v := valueView{deleted: deletedValues(log, "statuses", "priorities")}
	var statuses []string
	for f := range v.deleted {
		if strings.HasPrefix(f, "statuses/") {
			statuses = append(statuses, f)
		}
	}
	v.all, _ = workspace.SettingsReviving(log, statuses...)
	fields := []pointer{{"initial_status", "statuses/" + string(live.InitialStatus)}}
	for _, c := range workspace.Categories {
		fields = append(fields, pointer{"category_default/" + string(c), "statuses/" + string(live.CategoryDefaults[c])})
	}
	fields = append(fields, pointer{"initial_priority", "priorities/" + string(live.InitialPriority)})
	for _, p := range fields {
		if len(v.deleted[p.F]) > 0 {
			v.pointers = append(v.pointers, p)
		}
	}
	return v, true
}

// deletedValues は、取り消されていない削除の行のうち、f が <list>/<ID>(list は lists のどれか)のものを、f ごとに集める。
func deletedValues(log []change.Record, lists ...string) map[string][]change.Record {
	restored := change.Restored(log)
	out := map[string][]change.Record{}
	for _, r := range log {
		list, id, ok := strings.Cut(r.F, "/")
		if r.T != change.Delete || restored(r) || !ok || id == "" || strings.Contains(id, "/") || !slices.Contains(lists, list) {
			continue
		}
		out[r.F] = append(out[r.F], r)
	}
	return out
}

// pointed は、f の値を指す項目があるか。
func (v valueView) pointed(f string) bool {
	return slices.ContainsFunc(v.pointers, func(p pointer) bool { return p.F == f })
}

// category は、ステータスの f(statuses/<ID>)の分類。削除済みのものも読む。
func (v valueView) category(f string) (workspace.Category, bool) {
	for _, s := range v.all.Statuses {
		if "statuses/"+string(s.ID) == f {
			return s.Category, true
		}
	}
	return "", false
}

// emptied は、分類 c のステータスが、削除済みを含めて 1 つ以上あり、すべて削除されていて、指されてもいない(決定 C5b2-4)
// ときの、c の削除されたステータスの f(並びの順)。そうでなければ nil。
func (v valueView) emptied(c workspace.Category) []string {
	var fs []string
	for _, s := range v.all.Statuses {
		if s.Category != c {
			continue
		}
		f := "statuses/" + string(s.ID)
		if len(v.deleted[f]) == 0 || v.pointed(f) {
			return nil
		}
		fs = append(fs, f)
	}
	return fs
}

// ReasonOf は、削除の行の f(statuses/<ID>・priorities/<ID>)の取り消しの理由(決定 C5b2-4・C5b2-9 の順)。
// 指している項目が先で、次に 0 個になる分類。f が削除されていない・どちらでもなければ false。
func ReasonOf(log []change.Record, f string) (RestoreReason, bool) {
	v, ok := readValues(log)
	if !ok || len(v.deleted[f]) == 0 {
		return RestoreReason{}, false
	}
	for _, p := range v.pointers {
		if p.F == f {
			return RestoreReason{Field: p.Field}, true
		}
	}
	if c, ok := v.category(f); ok && slices.Contains(v.emptied(c), f) {
		return RestoreReason{Category: c}, true
	}
	return RestoreReason{}, false
}

// ValueRestores は、取り消す値の削除(決定 C5b2-4)。指されている値は、それを覆う削除をすべて取り消す。
// 0 個になる分類は、その分類の削除の行のうち、Facts の自分の側のもの(なければすべて)から at が最もあとの行
// (同じなら ID の大きい行)を選び、その値を覆う削除をすべて取り消す。指されているものを先に数える
// (指されている値は、その分類に残るものに数える)。結果は Op の辞書順で、同じ op は 1 つ(f の小さい方)。
func ValueRestores(log []change.Record, f Facts) []Overlap {
	v, ok := readValues(log)
	if !ok {
		return nil
	}
	byOp := map[change.ID]Overlap{}
	add := func(fs string) {
		for _, d := range v.deleted[fs] {
			if cur, ok := byOp[d.Op]; !ok || d.F < cur.F {
				byOp[d.Op] = Overlap{Op: d.Op, F: d.F}
			}
		}
	}
	for _, p := range v.pointers {
		add(p.F)
	}
	for _, c := range workspace.Categories {
		if fs := v.emptied(c); len(fs) > 0 {
			add(v.pick(fs, f).F)
		}
	}
	out := make([]Overlap, 0, len(byOp))
	for _, o := range byOp {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Op < out[j].Op })
	return out
}

// pick は、fs の値の削除の行から、0 個になる分類で取り消す 1 行を選ぶ。
func (v valueView) pick(fs []string, f Facts) change.Record {
	var all, ours *change.Record
	for _, x := range fs {
		for i := range v.deleted[x] {
			d := &v.deleted[x][i]
			all = later(all, d)
			if f.Side(d.ID) == 1 {
				ours = later(ours, d)
			}
		}
	}
	if ours != nil {
		return *ours
	}
	return *all
}

// FinishedInitial は、初期ステータス X の分類が完了・中止なら、X を未着手の既定に戻す set initial_status(06「完了・中止の
// 分類にされた初期ステータス」)。X がいまあるステータスでない・未着手の既定がない(いまあるステータスでない)・
// X と同じなら false。
func FinishedInitial(log []change.Record) (change.Draft, bool) {
	st, ok := workspace.SettingsFromLog(log)
	if !ok || !st.Finished(st.InitialStatus) {
		return change.Draft{}, false
	}
	todo := st.CategoryDefaults[workspace.CategoryTodo]
	live := slices.ContainsFunc(st.Statuses, func(s workspace.Value) bool { return s.ID == todo })
	if !live || todo == st.InitialStatus {
		return change.Draft{}, false
	}
	return change.NewSet("initial_status", st.InitialStatus, todo), true
}
