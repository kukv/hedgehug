package repair

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Numbered は、番号を持つアイテムまたはプロジェクト 1 つ。Create は、その作成の行。
type Numbered struct {
	ID     change.ID
	Number int
	Create change.Record
}

// Renumber は、番号を From から To に振り直すもの 1 つ。
type Renumber struct {
	ID       change.ID
	From, To int
}

// Renumbers は、同じワークスペースで番号が重なったものの振り直しを返す(決定 C3a-13)。
// 重なった番号ごとに、残す 1 件を決め、ほかを振り直す。残すのは、自分の側(Ours)で作っていないものの中で
// 作成が最も旧い 1 件(時刻が同じなら内部 ID が前の方)。Ours で作っていないものがなければ、全体で最も旧い 1 件。
// 振り直す順は、旧い番号の小さい順、同じ番号の中は内部 ID の順(時刻には頼らない)。To は next から 1 つずつ。
// 重ならなければ空。
func Renumbers(all []Numbered, f Facts, next int) []Renumber {
	groups := map[int][]Numbered{}
	for _, n := range all {
		groups[n.Number] = append(groups[n.Number], n)
	}
	numbers := make([]int, 0, len(groups))
	for n, g := range groups {
		if len(g) > 1 {
			numbers = append(numbers, n)
		}
	}
	slices.Sort(numbers)
	var out []Renumber
	for _, n := range numbers {
		for _, m := range moved(groups[n], f) {
			out = append(out, Renumber{ID: m.ID, From: n, To: next + len(out)})
		}
	}
	return out
}

// moved は、番号が重なった g のうち、振り直すものを、振り直す順で返す。
func moved(g []Numbered, f Facts) []Numbered {
	older := func(a, b Numbered) int {
		if c := a.Create.At.Compare(b.Create.At); c != 0 {
			return c
		}
		return strings.Compare(string(a.ID), string(b.ID))
	}
	pool := slices.DeleteFunc(slices.Clone(g), func(n Numbered) bool { return f.Side(n.Create.ID) == 1 })
	if len(pool) == 0 {
		pool = g
	}
	keep := slices.MinFunc(pool, older)
	var out []Numbered
	for _, n := range g {
		if n.ID != keep.ID {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, func(a, b Numbered) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return out
}
