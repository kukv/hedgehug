package projects

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// MoveProject は、プロジェクトを、ワークスペースの中で 1 つ上(up)か下へ動かす(04「並び順と並べ替え」)。
// 端なら何も保存しない。位置が変わるのは動かした 1 件だけで、その rank を新しい隣どうしのあいだの値にする。
// rank のないものがあれば、同じ保存で、表示の順を保つ rank を先に振る。削除済み(root: true)なら、先に削除を取り消す。
// アクティビティは「{タイトル} の並びを変更」。
func (s Service) MoveProject(ctx context.Context, store port.StoreID, id project.ID, up bool) error {
	p, err := s.editable(ctx, store, id)
	if err != nil {
		return err
	}
	summary := fmt.Sprintf("%s の並びを変更", label(p.Title))
	_, err = s.saveAs(ctx, store, id, "", summary, func(tx port.Tx, cur project.Project) ([]change.Draft, error) {
		list, err := port.LiveProjects(tx, cur.Place.Workspace)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(list, func(q project.Project) bool { return q.ID == id })
		if i < 0 {
			return nil, port.ErrNotFound
		}
		if (up && i == 0) || (!up && i == len(list)-1) {
			return nil, nil
		}
		ranks, err := fillProjectRanks(tx, list)
		if err != nil {
			return nil, err
		}
		return []change.Draft{change.NewSet("rank", ranks[i], rank.Moved(ranks, i, up))}, nil
	})
	return err
}

// fillProjectRanks は、rank が空か、前のもの以下(同じ値を含む)のプロジェクトだけに、表示の順を保つ rank を振り
// (rank.Refill)、同じ保存に書く。振ったあとの並びを返す。
func fillProjectRanks(tx port.Tx, list []project.Project) ([]string, error) {
	ranks := make([]string, len(list))
	for i, p := range list {
		ranks[i] = p.Rank
	}
	out, changed := rank.Refill(ranks)
	for i, p := range list {
		if !changed[i] {
			continue
		}
		var from any
		if p.Rank != "" {
			from = p.Rank
		}
		if err := tx.WriteProjects().Append(p.ID, change.NewSet("rank", from, out[i])); err != nil {
			return nil, err
		}
	}
	return out, nil
}
