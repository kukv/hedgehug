// Package textmerge は、説明(description.md)の 3 方向の行マージの規則を持つ。入出力をしない。
// 保存(app)と同期(infra)が同じ関数を使う(06「同時に編集したとき」)。
package textmerge

import (
	"slices"
	"strings"
)

// conflictSeparator は、同じ箇所を両方が変えたときに、2 つの版のあいだにはさむ行(06「同時に編集したとき」)。
const conflictSeparator = "-----"

// MergeText は、Merge の合わせた文字列だけを返す。
func MergeText(base, older, newer string) string {
	merged, _ := Merge(base, older, newer)
	return merged
}

// Merge は、base から older と newer に分かれた説明を、行ごとに合わせる。別々の行の変更は両方残す。
// 同じ箇所を両方が変えたときは、older の版、「-----」、newer の版の順に両方を残し、conflict を true にする。
func Merge(base, older, newer string) (merged string, conflict bool) {
	switch {
	case older == newer, newer == base:
		return older, false
	case older == base:
		return newer, false
	}
	b, o, n := strings.Split(base, "\n"), strings.Split(older, "\n"), strings.Split(newer, "\n")
	hunks := slices.Concat(diffHunks(b, o, true), diffHunks(b, n, false))
	slices.SortStableFunc(hunks, func(x, y hunk) int {
		if c := x.start - y.start; c != 0 {
			return c
		}
		return x.end - y.end
	})
	var out []string
	pos := 0
	for _, c := range clusters(hunks) {
		out = append(out, b[pos:c.start]...)
		chunk, both := mergeChunk(b[c.start:c.end], c.apply(b, true), c.apply(b, false))
		out = append(out, chunk...)
		conflict = conflict || both
		pos = c.end
	}
	out = append(out, b[pos:]...)
	return strings.Join(out, "\n"), conflict
}

// hunk は、base の [start, end) の行を、片方の版が repl に置き換えたことを表す(start == end なら挿入)。
type hunk struct {
	start, end int
	repl       []string
	isOlder    bool
}

// diffHunks は、base と other の差を、安定な行(最長共通部分列)のあいだの区間ごとにまとめる。
func diffHunks(base, other []string, isOlder bool) []hunk {
	match := matchLines(base, other)
	var hunks []hunk
	bi, oi := 0, 0
	for k := 0; k <= len(base); k++ {
		j := len(other)
		if k < len(base) {
			if match[k] < 0 {
				continue
			}
			j = match[k]
		}
		if k > bi || j > oi {
			hunks = append(hunks, hunk{start: bi, end: k, repl: other[oi:j], isOlder: isOlder})
		}
		bi, oi = k+1, j+1
	}
	return hunks
}

// cluster は、base の行が重なる(または同じ位置への挿入が重なる)変更をひとまとめにした区間。
type cluster struct {
	start, end int
	hunks      []hunk
}

// clusters は、start 順に並べた変更を、重なるものどうしでまとめる。重ならない変更は別の区間になり、両方が残る。
func clusters(hunks []hunk) []cluster {
	var list []cluster
	for _, h := range hunks {
		if last := len(list) - 1; last >= 0 && joins(list[last], h) {
			list[last].end = max(list[last].end, h.end)
			list[last].hunks = append(list[last].hunks, h)
			continue
		}
		list = append(list, cluster{start: h.start, end: h.end, hunks: []hunk{h}})
	}
	return list
}

func joins(c cluster, h hunk) bool {
	return h.start < c.end || (h.start == c.end && c.start == c.end && h.start == h.end)
}

// apply は、区間の base の行に、片方の版の変更だけを当てた結果を返す。
func (c cluster) apply(base []string, isOlder bool) []string {
	var out []string
	pos := c.start
	for _, h := range c.hunks {
		if h.isOlder != isOlder {
			continue
		}
		out = append(out, base[pos:h.start]...)
		out = append(out, h.repl...)
		pos = h.end
	}
	return append(out, base[pos:c.end]...)
}

// mergeChunk は、安定な行のあいだの 1 区間を合わせる。両方を残したときは true。
func mergeChunk(base, o, n []string) ([]string, bool) {
	switch {
	case slices.Equal(o, n), slices.Equal(n, base):
		return o, false
	case slices.Equal(o, base):
		return n, false
	}
	// 説明のなかった末尾に両方が書いたときなど、空の行だけの版は無視しない。
	return slices.Concat(o, []string{conflictSeparator}, n), true
}

// matchLines は、最長共通部分列で base の各行に対応する other の行の位置を返す(対応しなければ -1)。
func matchLines(base, other []string) []int {
	lcs := lcsTable(base, other)
	match := make([]int, len(base))
	i, j := 0, 0
	for i < len(base) && j < len(other) {
		switch {
		case base[i] == other[j]:
			match[i] = j
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			match[i] = -1
			i++
		default:
			j++
		}
	}
	for ; i < len(base); i++ {
		match[i] = -1
	}
	return match
}

// lcsTable は、base[i:] と other[j:] の最長共通部分列の長さの表を作る。
func lcsTable(base, other []string) [][]int {
	lcs := make([][]int, len(base)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(other)+1)
	}
	for i := len(base) - 1; i >= 0; i-- {
		for j := len(other) - 1; j >= 0; j-- {
			if base[i] == other[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	return lcs
}
