package repair

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// foldKey は、strings.EqualFold で等しい文字列に同じ値を返す。各文字を SimpleFold の輪の最小の文字にそろえる。
func foldKey(s string) string {
	return strings.Map(func(r rune) rune {
		m := r
		for c := unicode.SimpleFold(r); c != r; c = unicode.SimpleFold(c) {
			m = min(m, c)
		}
		return m
	}, s)
}

// prefixMax は、接頭辞の最大の長さ(workspace.ValidPrefix と同じ)。
const prefixMax = 10

// Named は、重なりを数える範囲の中の、名前・接頭辞・キーを持つもの 1 つ。
// Value は前後の空白を除いた値、Naming は、その値を決めた行(決定 C5c1-3)。
type Named struct {
	Ref    change.Ref
	Field  string
	Value  string
	Naming change.Record
}

// Rename は、重なった Named のうち、To に直すもの 1 つ。
type Rename struct {
	Named
	To string
}

// NamingRow は、ログの中で項目 f の値を決めた行を返す(決定 C5c1-3)。
// f の set の先端(change.Current)があればその行、なければ項目を作った最初の create の行。どちらもなければ false。
func NamingRow(log []change.Record, f string) (change.Record, bool) {
	if r, ok := change.Current(log, change.Set, f, func(change.Record) bool { return true }); ok {
		return r, true
	}
	for _, r := range log {
		if r.T == change.Create {
			return r, true
		}
	}
	return change.Record{}, false
}

// older は、重なった 2 件の並び(決定 C5c1-4・C5c2-4)。自分の側(Facts.Side == 1)で足していない行のものが先、
// 次に行の at が古いもの、同じなら tie の文字列が小さいもの。row は、残す 1 件を決める行。
// tie は、番号なら内部 ID(C3a-13)、名前なら名前を決めた行の ID(C5c1-4)。
func older[T any](row func(T) change.Record, tie func(T) string, f Facts) func(a, b T) int {
	ours := func(n T) bool { return f.Side(row(n).ID) == 1 }
	return func(a, b T) int {
		if oa, ob := ours(a), ours(b); oa != ob {
			if oa {
				return 1
			}
			return -1
		}
		if c := row(a).At.Compare(row(b).At); c != 0 {
			return c
		}
		return strings.Compare(tie(a), tie(b))
	}
}

// keeper は、重なった g のうち、残す 1 件を返す(決定 C5c1-4、C3a-13)。older で並べた先頭。
// どれも自分の側なら、g 全体で最も古いもの。g は空でないこと。
func keeper[T any](g []T, row func(T) change.Record, tie func(T) string, f Facts) T {
	return slices.MinFunc(g, older(row, tie, f))
}

// NameRenames は、重なった名前の直しを返す(決定 C5c1-4・C5c1-5)。新しい値は「{値} (n)」。
// fold が真なら大文字と小文字を区別せずに比べる。重ならなければ空。
func NameRenames(all []Named, f Facts, fold bool) []Rename {
	same := func(v string) string { return v }
	if fold {
		same = foldKey
	}
	return renames(all, f, same, func(v string, n int) string { return v + " (" + strconv.Itoa(n) + ")" })
}

// PrefixRenames は、重なった接頭辞の直しを返す。新しい値は「{値}{n}」で、10 文字を超えるときは値の末尾を削る。
func PrefixRenames(all []Named, f Facts) []Rename {
	return renames(all, f, func(v string) string { return v }, func(v string, n int) string {
		suffix := strconv.Itoa(n)
		return v[:min(len(v), prefixMax-len(suffix))] + suffix
	})
}

// KeyRenames は、重なったプロパティのキーの直しを返す。新しい値は「{値}{n}」。
func KeyRenames(all []Named, f Facts) []Rename {
	return renames(all, f, func(v string) string { return v }, func(v string, n int) string { return v + strconv.Itoa(n) })
}

// renames は、same で比べて重なった組ごとに、残す 1 件を keeper で決め、
// ほかを内部 ID の順に、n = 2 から試して重ならない最初の値に直す。組は値の辞書順。
func renames(all []Named, f Facts, same func(string) string, candidate func(v string, n int) string) []Rename {
	groups := map[string][]Named{}
	taken := map[string]bool{}
	for _, n := range all {
		k := same(n.Value)
		groups[k] = append(groups[k], n)
		taken[k] = true
	}
	keys := make([]string, 0, len(groups))
	for k, g := range groups {
		if len(g) > 1 {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var out []Rename
	for _, k := range keys {
		g := groups[k]
		keep := keeper(g, func(n Named) change.Record { return n.Naming }, func(n Named) string { return string(n.Naming.ID) }, f)
		rest := slices.DeleteFunc(slices.Clone(g), func(n Named) bool { return n.Ref == keep.Ref })
		slices.SortFunc(rest, func(a, b Named) int { return strings.Compare(string(a.Ref.ID), string(b.Ref.ID)) })
		for _, n := range rest {
			to := firstFree(n.Value, taken, same, candidate)
			taken[same(to)] = true
			out = append(out, Rename{Named: n, To: to})
		}
	}
	return out
}

// firstFree は、n = 2 から順に candidate を作り、taken に含まれない最初の値を返す。
func firstFree(v string, taken map[string]bool, same func(string) string, candidate func(string, int) string) string {
	for n := 2; ; n++ {
		if to := candidate(v, n); !taken[same(to)] {
			return to
		}
	}
}
