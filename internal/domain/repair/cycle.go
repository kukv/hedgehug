package repair

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// RepairEpicTitle は、行き先の Epic がないときに作る Epic のタイトル(08「親がない」の呼び名。決定 C3a-15)。
const RepairEpicTitle = "修復で作った Epic"

// Cycle は、削除されていないアイテムだけで親をたどり、自分に戻る輪を 1 つ、内部 ID の順で返す
// (保存形式「同期の修復」の (4)。決定 C3a-14)。輪が複数あれば、輪の中の最小の内部 ID が小さい輪を返す。
// 削除済みの親や、ないアイテムに着いたら輪ではない。輪へ続く尾は輪に入れない。輪がなければ false。
func Cycle(items []item.Item) (ring []item.ID, ok bool) {
	live := item.Live(items)
	parent := make(map[item.ID]item.ID, len(live))
	ids := make([]item.ID, 0, len(live))
	for _, it := range live {
		ids = append(ids, it.ID)
		if it.Place.Parent != nil {
			parent[it.ID] = *it.Place.Parent
		}
	}
	slices.Sort(ids)
	// 内部 ID の順に、自分に戻るかを見る。最初に見つかったものは、その輪の最小の内部 ID。
	// 輪に入らないと分かったものは noRing に覚え、たどり直さない。
	noRing := map[item.ID]bool{}
	for _, id := range ids {
		if ring := ringFrom(id, parent, noRing); ring != nil {
			slices.Sort(ring)
			return ring, true
		}
	}
	return nil, false
}

// ringFrom は、start から親をたどって start に戻れば、通ったものを返す。戻らなければ nil。
// 親がないか輪に入らないものに着いたら、通ったものはどれも輪に入らないので noRing に入れる。
// start に戻らずに通ったものに戻ったら、start はほかの輪へ続く尾なので、start だけを noRing に入れる。
func ringFrom(start item.ID, parent map[item.ID]item.ID, noRing map[item.ID]bool) []item.ID {
	path := []item.ID{start}
	seen := map[item.ID]bool{start: true}
	for cur := start; ; {
		next, ok := parent[cur]
		switch {
		case !ok || noRing[next]:
			for _, id := range path {
				noRing[id] = true
			}
			return nil
		case next == start:
			return path
		case seen[next]:
			noRing[start] = true
			return nil
		}
		seen[next] = true
		path = append(path, next)
		cur = next
	}
}

// Mover は、輪の中で動かすもの。番号が最も大きいもの、同じなら内部 ID の辞書順であとのもの(決定 C3a-14)。
func Mover(ring []item.ID, items []item.Item) item.ID {
	number := map[item.ID]int{}
	for _, it := range items {
		number[it.ID] = it.Place.Number
	}
	return slices.MaxFunc(ring, func(a, b item.ID) int {
		return cmp.Or(cmp.Compare(number[a], number[b]), strings.Compare(string(a), string(b)))
	})
}

// Destination は、動かす行き先。Parent が nil ならプロジェクトの直下。NeedEpic なら、行き先の Epic を作る。
type Destination struct {
	Parent   *item.ID
	NeedEpic bool
}

// DestinationOf は、it の行き先(08「親がない」。決定 C3a-14)。Epic ならプロジェクトの直下。それ以外は、
// it のプロジェクトの削除されていない親のない Epic(it を除く)を item.SortSiblings で並べた先頭。なければ NeedEpic。
func DestinationOf(it item.Item, items []item.Item) Destination {
	if it.Kind == item.Epic {
		return Destination{}
	}
	var epics []item.Item
	for _, e := range item.Live(items) {
		if e.Kind == item.Epic && e.Place.Parent == nil && e.Place.Project == it.Place.Project && e.ID != it.ID {
			epics = append(epics, e)
		}
	}
	if len(epics) == 0 {
		return Destination{NeedEpic: true}
	}
	item.SortSiblings(epics)
	return Destination{Parent: &epics[0].ID}
}

// EpicID は、プロジェクト p に作る「修復で作った Epic」の内部 ID(決定 C3a-15)。p の先頭 1 文字を R に替えたもの。
// used がその ID を使っているとき(削除済みを含む)は、S、T … Z の順に替える。すべて使われていれば空。
func EpicID(p project.ID, used func(change.ID) bool) change.ID {
	for c := 'R'; c <= 'Z'; c++ {
		id := change.ID(string(c) + string(p[1:]))
		if !used(id) {
			return id
		}
	}
	return ""
}
