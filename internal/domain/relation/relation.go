// Package relation は、アイテムの関連(02「関連」)の規則を持つ: 9 つの表記、一覧、循環、ブロック中。
// 関連はワークスペースをまたぐので、呼ぶ側が Store のすべてのアイテムを渡す。
package relation

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/item"
)

// Label は、画面の表記。
type Label string

const (
	Blocks       Label = "blocks"
	BlockedBy    Label = "blocked by"
	Causes       Label = "causes"
	CausedBy     Label = "caused by"
	Duplicates   Label = "duplicates"
	DuplicatedBy Label = "duplicated by"
	RelatesTo    Label = "relates to"
	Clones       Label = "clones"
	ClonedBy     Label = "cloned by"
)

// Labels は、一覧の見出しの順(02「関連」の「一覧」)。
var Labels = []Label{Blocks, BlockedBy, Causes, CausedBy, Duplicates, DuplicatedBy, RelatesTo, Clones, ClonedBy}

// TypeBlocks は、循環の検査と「ブロック中」に使う種類。
const TypeBlocks = "blocks"

type kind struct {
	typ     string
	reverse bool // 持たれる側(相手が持つ)から見た表記
}

var kinds = map[Label]kind{
	Blocks: {"blocks", false}, BlockedBy: {"blocks", true},
	Causes: {"causes", false}, CausedBy: {"causes", true},
	Duplicates: {"duplicates", false}, DuplicatedBy: {"duplicates", true},
	RelatesTo: {"relates", false},
	Clones:    {"clones", false}, ClonedBy: {"clones", true},
}

// Parse は、表記を、保存する種類と、自分が持たれる側(逆向きの表記)かに分ける。知らない表記は ok=false。
func Parse(l Label) (typ string, reverse bool, ok bool) {
	k, ok := kinds[l]
	return k.typ, k.reverse, ok
}

// LabelOf は、保存した種類を、持つ側(reverse=false)か持たれる側(true)から見た表記にする。
// relates には向きがなく、どちらからも relates to。知らない種類は ok=false。
func LabelOf(typ string, reverse bool) (Label, bool) {
	if typ == "relates" {
		return RelatesTo, true
	}
	for _, l := range Labels {
		if k := kinds[l]; k.typ == typ && k.reverse == reverse {
			return l, true
		}
	}
	return "", false
}

// Row は、self から見た関連の 1 行。
type Row struct {
	Label Label
	Other item.ID
}

// Rows は、self の関連を一覧の順(表記の順、同じ表記では相手の内部 ID の順)で返す。self が持つ関連は自分の側の
// 表記、self を相手にした関連は逆向きの表記で出す。知らない種類と、相手が all にいない関連は出さない。
func Rows(self item.ID, all []item.Item) []Row {
	exists := make(map[item.ID]bool, len(all))
	for _, it := range all {
		exists[it.ID] = true
	}
	seen := map[Row]bool{}
	var rows []Row
	add := func(l Label, ok bool, other item.ID) {
		r := Row{Label: l, Other: other}
		if ok && exists[other] && !seen[r] {
			seen[r] = true
			rows = append(rows, r)
		}
	}
	for _, it := range all {
		for _, rel := range it.Relations {
			switch self {
			case it.ID:
				l, ok := LabelOf(rel.Type, false)
				add(l, ok, rel.To)
			case rel.To:
				l, ok := LabelOf(rel.Type, true)
				add(l, ok, it.ID)
			}
		}
	}
	slices.SortStableFunc(rows, func(a, b Row) int {
		if c := slices.Index(Labels, a.Label) - slices.Index(Labels, b.Label); c != 0 {
			return c
		}
		return strings.Compare(string(a.Other), string(b.Other))
	})
	return rows
}

// Has は、a と b のあいだに種類 typ の関連があるか。向きは問わない(02「関連」の「追加」。種類ごとに 1 つまで)。
func Has(a, b item.ID, typ string, all []item.Item) bool {
	for _, it := range all {
		for _, rel := range it.Relations {
			forward := it.ID == a && rel.To == b
			backward := it.ID == b && rel.To == a
			if rel.Type == typ && (forward || backward) {
				return true
			}
		}
	}
	return false
}

// WouldCycle は、from が to をブロックする関連を足すと、blocks の輪になるか(to から blocks をたどって from に
// 着くか)。from と to が同じなら輪。
func WouldCycle(from, to item.ID, all []item.Item) bool {
	next := map[item.ID][]item.ID{}
	for _, it := range all {
		for _, rel := range it.Relations {
			if rel.Type == TypeBlocks {
				next[it.ID] = append(next[it.ID], rel.To)
			}
		}
	}
	seen := map[item.ID]bool{}
	stack := []item.ID{to}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == from {
			return true
		}
		if !seen[cur] {
			seen[cur] = true
			stack = append(stack, next[cur]...)
		}
	}
	return false
}

// BlockedSet は、ブロック中のアイテムの内部 ID。ブロックしている相手のうち、closed(完了か中止)でないものが
// 1 つでもあるとき(02「上から順の構成」)。削除済みを除くのは呼ぶ側(A4b)。
func BlockedSet(all []item.Item, closed func(item.Item) bool) map[item.ID]bool {
	out := map[item.ID]bool{}
	for _, it := range all {
		if closed(it) {
			continue
		}
		for _, rel := range it.Relations {
			if rel.Type == TypeBlocks {
				out[rel.To] = true
			}
		}
	}
	return out
}

// Blocked は、self がブロック中か。
func Blocked(self item.ID, all []item.Item, closed func(item.Item) bool) bool {
	return BlockedSet(all, closed)[self]
}
