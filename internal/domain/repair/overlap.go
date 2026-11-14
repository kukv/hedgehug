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

// Changer は、削除の行 d と重なった変更をした人のログイン名(決定 C4b2-9)。LastChange の行の by。見つからなければ空。
func Changer(log []change.Record, d change.Record, f Facts) string {
	if w, ok := LastChange(log, d, f); ok {
		return w.By
	}
	return ""
}

// LastChange は、削除の行 d と重なった変更のうち最もあとの行(決定 C4b2-9)。hasChange と同じ数え方: d の op でない、
// 人が書いた変更の行(delete・restore を除く)のうち、d の f が deleted ならすべての行、ほかは f の下の f(d.F+"/" で
// 始まるもの)の行。作成と moved_in の行は、削除より前にしかない(削除と同時にはできない)ので数えない。f で d が
// 片側にあれば、もう片側の行だけを数える(同時に変えた人。共通の祖先にある変更は数えない)。d がどちらの側にも
// なければ(f が空のときも)、側を見ない。削除した人(d.By。大文字と小文字を区別しない)でない人の行のうち at が
// 最もあとの行(同じなら ID の大きい行)を返し、削除した人の行しかなければ、その中で最もあとの行を返す。
func LastChange(log []change.Record, d change.Record, f Facts) (change.Record, bool) {
	side := f.Side(d.ID)
	var other, own *change.Record
	for i := range log {
		c := &log[i]
		if c.Op == d.Op || c.Actor.Kind != change.ActorUser || slices.Contains(notChanges, c.T) {
			continue
		}
		if (d.F != change.DeletedField && !strings.HasPrefix(c.F, d.F+"/")) || (side != 0 && f.Side(c.ID) != 3-side) {
			continue
		}
		if strings.EqualFold(c.By, d.By) {
			own = later(own, c)
		} else {
			other = later(other, c)
		}
	}
	switch {
	case other != nil:
		return *other, true
	case own != nil:
		return *own, true
	}
	return change.Record{}, false
}

// later は、w と c のうち at があとの行(同じなら ID の大きい行)。w が nil なら c。
func later(w, c *change.Record) *change.Record {
	if w == nil || c.At.After(w.At) || (c.At.Equal(w.At) && c.ID > w.ID) {
		return c
	}
	return w
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
