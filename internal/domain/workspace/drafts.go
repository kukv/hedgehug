package workspace

import (
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// PropertyDrafts は、作成後に足すプロパティの行(B2-5)。使う種類は 1 つずつ add。
func PropertyDrafts(p Property) []change.Draft {
	base := "properties/" + string(p.ID) + "/"
	ds := []change.Draft{
		change.NewSet(base+"key", nil, p.Key),
		change.NewSet(base+"name", nil, p.Name),
		change.NewSet(base+"type", nil, p.Type),
		change.NewSet(base+"builtin", nil, false),
		change.NewSet(base+"hidden", nil, false),
		change.NewSet(base+"rank", nil, p.Rank),
	}
	for _, k := range p.Kinds {
		ds = append(ds, change.NewAdd(base+"kinds", k))
	}
	return ds
}

// ValueDrafts は、一覧 l に足す値の行。分類はステータスだけ。
func ValueDrafts(l List, v Value) []change.Draft {
	ds := []change.Draft{
		change.NewSet(l.Field(v.ID, "name"), nil, v.Name),
		change.NewSet(l.Field(v.ID, "color"), nil, v.Color),
		change.NewSet(l.Field(v.ID, "rank"), nil, v.Rank),
	}
	if l.Kind == ListStatuses {
		ds = append(ds, change.NewSet(l.Field(v.ID, "category"), nil, v.Category))
	}
	return ds
}

// SwapRanks は、隣どうしの並びの入れ替え(B2-6)。fa の順位 ra と fb の順位 rb を取り替える。
func SwapRanks(fa, ra, fb, rb string) []change.Draft {
	return []change.Draft{change.NewSet(fa, ra, rb), change.NewSet(fb, rb, ra)}
}

// NextRank は、ranks のどれよりもあとに並ぶ値。ranks が空なら rank.First。
func NextRank(ranks ...string) string {
	last := ""
	for _, r := range ranks {
		last = max(last, r)
	}
	return rank.After(last)
}
