package repair

import (
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
// incidental は、ほかのものへの操作で一緒に書いた行かを答える(nil なら、そういう行はない)。
// 結果は Op の辞書順で、同じ op は 1 つ。
func Overlaps(log []change.Record, f Facts, incidental func(change.Record) bool) []Overlap {
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

// hasChange は、d と重なる変更が、side の側にあるか。
func hasChange(log []change.Record, d change.Record, side int, f Facts, incidental func(change.Record) bool) bool {
	for _, c := range log {
		if f.Side(c.ID) != side || c.T == change.Delete || c.T == change.Restore || c.Op == d.Op {
			continue
		}
		if c.Actor.Kind != change.ActorUser || (incidental != nil && incidental(c)) {
			continue
		}
		if d.F == change.DeletedField || strings.HasPrefix(c.F, d.F+"/") {
			return true
		}
	}
	return false
}
