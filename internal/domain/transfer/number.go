package transfer

import "github.com/kukv/hedgehug/internal/domain/item"

// Number は、移動先のワークスペースでの番号を決める。移すアイテム(root)が next で、続いて子・孫を並び順に深さ優先。
// 削除済みの子は、同じ親の下の削除されていない子のあと(削除済みどうしは並び順)。below の並びは信用しない。
func Number(root item.Item, below []item.Item, next int) map[item.ID]int {
	children := map[item.ID][]item.Item{}
	for _, it := range below {
		if it.Place.Parent != nil {
			children[*it.Place.Parent] = append(children[*it.Place.Parent], it)
		}
	}
	out := map[item.ID]int{root.ID: next}
	n := next
	var walk func(parent item.ID)
	walk = func(parent item.ID) {
		kids := children[parent]
		item.SortSiblings(kids)
		for _, deleted := range []bool{false, true} {
			for _, k := range kids {
				if _, seen := out[k.ID]; seen || (k.Deleted != nil) != deleted {
					continue
				}
				n++
				out[k.ID] = n
				walk(k.ID)
			}
		}
	}
	walk(root.ID)
	return out
}
