package transfers

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workRanks は、別のワークスペース ws へ移す moved の、移動先での順位(F2a-6)。dstItems は移動先のワークスペースのアイテム。
// moved の作業項目は、移動元の並び順のまま、移動先の作業項目の末尾に続く。root は移すアイテム(プロジェクトの移動は空)で、
// siblingEnd は移動先の親の削除されていない子の末尾。root が作業項目なら、その順位は親の子の末尾も満たす(F2a-5 の値から
// 続ける)。作業項目でない root(Epic)は親の子の末尾。ほかのもの(Step)は入らない。
func workRanks(ws workspace.ID, dstItems, moved []item.Item, root *item.Item, siblingEnd string) map[item.ID]string {
	start := planning.WorkEnd(dstItems, ws)
	if root != nil && planning.IsWork(root.Kind) {
		start = max(start, siblingEnd)
	}
	ranks := planning.AppendWorks(moved, start)
	if root != nil && !planning.IsWork(root.Kind) {
		ranks[root.ID] = rank.After(siblingEnd)
	}
	return ranks
}

// workspaceItems は、ワークスペース ws の全プロジェクトのアイテム(削除済みも含む)。
func workspaceItems(r port.Reader, ws workspace.ID) ([]item.Item, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var all []item.Item
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		all = append(all, its...)
	}
	return all, nil
}

// rankLine は、順位を ranks の値へ変える行。ranks にないか、今と同じなら書かない。
func rankLine(it item.Item, ranks map[item.ID]string) []change.Draft {
	r, ok := ranks[it.ID]
	if !ok || r == it.Rank {
		return nil
	}
	return []change.Draft{change.NewSet("rank", it.Rank, r)}
}
