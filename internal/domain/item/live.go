package item

// Live は、削除済み(Deleted がある)を除いたアイテムを、順を保って返す。
func Live(its []Item) []Item {
	out := make([]Item, 0, len(its))
	for _, it := range its {
		if it.Deleted == nil {
			out = append(out, it)
		}
	}
	return out
}

// Descendants は、id の子と孫を、深さ優先で返す。兄弟は SortSiblings の順。id 自身は含まない。
// 削除済みも含むので、呼ぶ側が必要なら Live を通す。親をたどると輪になっていても、同じものを 2 度は返さない。
func Descendants(id ID, all []Item) []Item {
	children := map[ID][]Item{}
	for _, it := range all {
		if it.Place.Parent != nil {
			children[*it.Place.Parent] = append(children[*it.Place.Parent], it)
		}
	}
	var out []Item
	seen := map[ID]bool{id: true}
	var walk func(parent ID)
	walk = func(parent ID) {
		kids := children[parent]
		SortSiblings(kids)
		for _, k := range kids {
			if seen[k.ID] {
				continue
			}
			seen[k.ID] = true
			out = append(out, k)
			walk(k.ID)
		}
	}
	walk(id)
	return out
}
