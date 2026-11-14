package query

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// sortVal は、並べ替えの値の 1 つ。キーごとに使う欄は決まっていて、s → f → i の順に比べる。
type sortVal struct {
	s string
	f float64
	i int64
}

func (a sortVal) compare(b sortVal) int {
	return cmp.Or(cmp.Compare(a.s, b.s), cmp.Compare(a.f, b.f), cmp.Compare(a.i, b.i))
}

// compareVals は、値の並びを先頭から比べ、先頭が同じなら値の数が少ない方を先にする。
func compareVals(a, b []sortVal) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := a[i].compare(b[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a), len(b))
}

type sorted[T any] struct {
	row  T
	f    *Facts
	vals []sortVal // 空は値がない
}

// Order は、rows をその場で並べ替える(E1-10)。s があれば全体をそのキーで並べ、値が同じなら Store の並び・
// ワークスペースの並び・アイテムの並び順の順。s がない、または表にないキーなら、その最後の 3 つだけで並べる。
// 値がない行は、昇順でも降順でも最後に置く。
func Order[T any](rows []T, facts func(T) *Facts, s *Sort) {
	entries := make([]sorted[T], len(rows))
	for i, r := range rows {
		entries[i] = sorted[T]{row: r, f: facts(r)}
	}
	desc := false
	if val := valueOf(entries, s); val != nil {
		desc = s.Desc
		for i := range entries {
			entries[i].vals = val(*entries[i].f)
		}
	}
	slices.SortStableFunc(entries, func(a, b sorted[T]) int {
		return cmp.Or(compareKey(a.vals, b.vals, desc), compareFacts(a.f, b.f))
	})
	for i, e := range entries {
		rows[i] = e.row
	}
}

// compareKey は、値の比べ方。値のないものを最後にし、降順は値の比べだけを逆にする。
func compareKey(a, b []sortVal, desc bool) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	case desc:
		return -compareVals(a, b)
	}
	return compareVals(a, b)
}

func compareFacts(a, b *Facts) int {
	return cmp.Or(
		cmp.Compare(a.StoreOrder, b.StoreOrder),
		cmp.Compare(a.WorkspaceOrder, b.WorkspaceOrder),
		item.CompareOrder(a.Item, b.Item),
	)
}

// valueOf は、キーの値を取り出す関数を返す。表にないキー(`s` が nil を含む)は nil で、既定の並びになる。
// 組み込みと同じ綴りの追加プロパティは組み込みを使う(組み込みで並べないキーは既定の並びのまま)。
func valueOf[T any](entries []sorted[T], s *Sort) func(Facts) []sortVal {
	if s == nil {
		return nil
	}
	if fn := builtinValue(s.Key, func() choiceRanks { return rankTable(entries, s.Key) }); fn != nil {
		return fn
	}
	if Builtin(s.Key) {
		return nil
	}
	return customValue(s.Key, func() choiceRanks { return rankTable(entries, s.Key) })
}

// choiceRanks は、選択肢の名前(小文字)→ ワークスペースをまたいだ定義順の位置。
type choiceRanks map[string]int

// rankTable は、rows に出てくるワークスペースを(Store、ワークスペース)の順に見て、各定義順で最初に現れた
// 位置で名前の順位を決める(04 の「並べ替え」。同じ名前は 1 つにする)。
func rankTable[T any](entries []sorted[T], key string) choiceRanks {
	ordered := make([]*Facts, 0, len(entries))
	for _, e := range entries {
		ordered = append(ordered, e.f)
	}
	slices.SortStableFunc(ordered, func(a, b *Facts) int {
		return cmp.Or(cmp.Compare(a.StoreOrder, b.StoreOrder), cmp.Compare(a.WorkspaceOrder, b.WorkspaceOrder))
	})
	type wsKey struct {
		store, ws int
		id        change.ID
	}
	seen := map[wsKey]bool{}
	ranks := choiceRanks{}
	for _, f := range ordered {
		k := wsKey{f.StoreOrder, f.WorkspaceOrder, f.Workspace.ID}
		if seen[k] {
			continue
		}
		seen[k] = true
		values := choiceValues(*f, key)
		for _, v := range values {
			name := strings.ToLower(v.Name)
			if _, ok := ranks[name]; !ok {
				ranks[name] = len(ranks)
			}
		}
	}
	return ranks
}

// choiceValues は、キーが指す選択肢の定義(ステータス・優先度・関連システム・選択式の追加プロパティ)。
func choiceValues(f Facts, key string) []workspace.Value {
	switch key {
	case "status":
		return f.Settings.Statuses
	case "priority":
		return f.Settings.Priorities
	case "system":
		return f.Settings.Systems
	}
	if p, ok := customProperty(f, key); ok && (p.Type == workspace.TypeSingle || p.Type == workspace.TypeMulti) {
		return p.Choices
	}
	return nil
}

// rankedVals は、ids の名前を表の順位にし、小さい順(ワークスペースの定義順)に並べ直す。
func rankedVals(ids []change.ID, values []workspace.Value, ranks choiceRanks) []sortVal {
	var out []sortVal
	for _, id := range ids {
		for _, v := range values {
			if v.ID == id {
				out = append(out, sortVal{f: float64(ranks[strings.ToLower(v.Name)])})
			}
		}
	}
	slices.SortFunc(out, sortVal.compare)
	return out
}
