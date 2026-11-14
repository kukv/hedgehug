package repair

import (
	"slices"
	"sort"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Overlap は、取り消す削除。Op は削除の op、F はその delete の行の f。
type Overlap struct {
	Op change.ID
	F  string
}

// Overlaps は、1 つのログから、削除と変更が重なったものを見つける(決定 C3a-9)。
// 削除の行が Facts の片側にあり、もう片側に、人が書いた変更の行がある削除のうち、まだ restore がないもの。
// incidental は、f(重なりを調べている delete の行の f)の削除にとって、ほかのものへの操作で一緒に書いた行かを
// 答える(nil なら、そういう行はない)。
// 結果は Op の辞書順で、同じ op は 1 つ。
func Overlaps(log []change.Record, f Facts, incidental func(f string, c change.Record) bool) []Overlap {
	restored := change.Restored(log)
	byOp := map[change.ID]Overlap{}
	for _, d := range log {
		side := f.Side(d.ID)
		if d.T != change.Delete || side == 0 || restored(d) || !deletesTarget(d) {
			continue
		}
		if !hasChange(log, d, 3-side, f, incidental) {
			continue
		}
		if cur, ok := byOp[d.Op]; !ok || d.F < cur.F {
			byOp[d.Op] = Overlap{Op: d.Op, F: d.F}
		}
	}
	var out []Overlap
	for _, o := range byOp {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Op < out[j].Op })
	return out
}

// deletesTarget は、取り消しの対象になる delete の行か。エンティティの削除は root: true のものだけ。
func deletesTarget(d change.Record) bool {
	if d.F != change.DeletedField {
		return true
	}
	return d.Root != nil && *d.Root
}

// notChanges は、Changer が変更に数えない行の種類。
var notChanges = []change.Type{change.Delete, change.Restore, change.Create, change.MovedIn}

// Changer は、削除の行 d と重なった変更をした人のログイン名(決定 C4b2-9)。hasChange と同じ数え方を、
// 片側の見分けなしで当てる: d の op でない、人が書いた変更の行(delete・restore を除く)のうち、d の f
// (deleted ならすべての f、ほかは f とその下の f)を変えたもので、at が最もあとの行(同じなら ID の大きい行)の by。
// 作成と moved_in の行は、削除より前にしかない(削除と同時にはできない)ので数えない。見つからなければ空。
func Changer(log []change.Record, d change.Record) string {
	var w *change.Record
	for i := range log {
		c := &log[i]
		if c.Op == d.Op || c.Actor.Kind != change.ActorUser || slices.Contains(notChanges, c.T) {
			continue
		}
		if d.F != change.DeletedField && c.F != d.F && !strings.HasPrefix(c.F, d.F+"/") {
			continue
		}
		if w == nil || c.At.After(w.At) || (c.At.Equal(w.At) && c.ID > w.ID) {
			w = c
		}
	}
	if w == nil {
		return ""
	}
	return w.By
}

// hasChange は、d と重なる変更が、side の側にあるか。
func hasChange(log []change.Record, d change.Record, side int, f Facts, incidental func(f string, c change.Record) bool) bool {
	for _, c := range log {
		if f.Side(c.ID) != side || c.T == change.Delete || c.T == change.Restore || c.Op == d.Op {
			continue
		}
		if c.Actor.Kind != change.ActorUser || (incidental != nil && incidental(d.F, c)) {
			continue
		}
		if d.F == change.DeletedField || strings.HasPrefix(c.F, d.F+"/") {
			return true
		}
	}
	return false
}
