package repair

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// UnderPurged は、祖先が完全に削除されたアイテムを、内部 ID の順で返す(保存形式「同期の修復」の (1)。決定 C3a-17)。
// 祖先は、親のアイテム(items にある限りたどる)と、各アイテムのプロジェクトと、そのワークスペース。
// そのどれかが purged にあるか、この結果に入るアイテムの下にあるものを返す。削除済みかは見ない。
// projectWorkspace にないプロジェクトは、そのワークスペースを見ない(プロジェクトの ID だけで判定する)。
// 親が循環していても止まる(輪の中だけでは、どれも入らない)。
func UnderPurged(items []item.Item, projectWorkspace map[project.ID]workspace.ID, purged map[change.ID]bool) []item.ID {
	byID := make(map[item.ID]item.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	var out []item.ID
	for _, it := range items {
		if underPurged(it, byID, projectWorkspace, purged) {
			out = append(out, it.ID)
		}
	}
	slices.Sort(out)
	return out
}

// underPurged は、it から親をたどり、どこかで完全に削除されたものに当たるか。この結果に入るアイテムの下に
// あることは、そのアイテムの祖先をたどり続けることで同じに分かる。
func underPurged(it item.Item, byID map[item.ID]item.Item, pw map[project.ID]workspace.ID, purged map[change.ID]bool) bool {
	seen := map[item.ID]bool{}
	for !seen[it.ID] {
		seen[it.ID] = true
		p := it.Place.Project
		if ws, ok := pw[p]; purged[p] || (ok && purged[ws]) {
			return true
		}
		if it.Place.Parent == nil {
			return false
		}
		parent := *it.Place.Parent
		if purged[parent] {
			return true
		}
		next, ok := byID[parent]
		if !ok {
			return false
		}
		it = next
	}
	return false
}

// Dangling は、完全に削除された相手への関連。Owner はその関連を持っているアイテム。
type Dangling struct {
	Owner    item.ID
	Relation item.Relation
}

// PurgedRelations は、相手が purged にある関連を、持ち主の内部 ID、相手の内部 ID、種類の順で返す
// (保存形式「同期の修復」の (1))。
func PurgedRelations(items []item.Item, purged map[change.ID]bool) []Dangling {
	var out []Dangling
	for _, it := range items {
		for _, rel := range it.Relations {
			if purged[rel.To] {
				out = append(out, Dangling{Owner: it.ID, Relation: rel})
			}
		}
	}
	slices.SortFunc(out, func(a, b Dangling) int {
		return cmp.Or(
			strings.Compare(string(a.Owner), string(b.Owner)),
			strings.Compare(string(a.Relation.To), string(b.Relation.To)),
			strings.Compare(a.Relation.Type, b.Relation.Type),
		)
	})
	return out
}
