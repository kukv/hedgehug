package item

import (
	"cmp"
	"slices"
	"strings"
)

// SortSiblings は、兄弟を並び順の値 → 作成の時刻 → 内部 ID の順に並べ替える(04「ツリー」の「行」)。
// 並び順の値が同じものは、作成の保存が早い方を先にし、同じなら内部 ID の辞書順にする。
func SortSiblings(its []Item) {
	slices.SortFunc(its, CompareOrder)
}

// CompareOrder は、SortSiblings と同じ比べ方(並び順の値 → 作成の時刻 → 内部 ID)で a と b を比べる。
func CompareOrder(a, b Item) int {
	return cmp.Or(
		strings.Compare(a.Rank, b.Rank),
		a.Created.At.Compare(b.Created.At),
		strings.Compare(string(a.ID), string(b.ID)),
	)
}
