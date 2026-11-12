package workspaces

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ranked は、並びの rank を添えたワークスペース。rank が空なら、まだ並びを持たない(B2-7)。
type ranked struct {
	workspace.Workspace
	rank string
}

// orderedWorkspaces は、ワークスペースを rank の順(同じなら内部 ID の順)で返す。rank のないものは先頭。
func orderedWorkspaces(r port.Reader) ([]ranked, error) {
	list, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	out := make([]ranked, 0, len(list))
	for _, w := range list {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		if err != nil {
			return nil, err
		}
		out = append(out, ranked{w, workspace.RankOf(log)})
	}
	slices.SortFunc(out, func(a, b ranked) int {
		return cmp.Or(cmp.Compare(a.rank, b.rank), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

// MoveWorkspace は、ワークスペースを 1 つ上(up)か下へ動かす(04「並び順と並べ替え」)。端なら何もしない。
// 位置が変わるのは動かした 1 件だけで、その rank を新しい隣どうしのあいだの値にする(B2-6)。
func (s Service) MoveWorkspace(ctx context.Context, store port.StoreID, ws workspace.ID, up bool) error {
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	d, err := s.Get(ctx, store, ws)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("%s の並びを変更", oneLine(d.Workspace.Name))}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		list, err := orderedWorkspaces(tx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(list, func(r ranked) bool { return r.ID == ws })
		if i < 0 {
			return port.ErrNotFound
		}
		if (up && i == 0) || (!up && i == len(list)-1) {
			return nil
		}
		if err := fillRanks(tx, list); err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(ws, change.NewSet("rank", list[i].rank, movedRank(list, i, up)))
	})
}

// movedRank は、list[i] を 1 つ上(up)か下へ動かしたときの rank。新しい隣どうしのあいだの値。
func movedRank(list []ranked, i int, up bool) string {
	if up {
		return rank.Between(rankAt(list, i-2), list[i-1].rank)
	}
	return rank.Between(list[i+1].rank, rankAt(list, i+2))
}

// rankAt は list[i] の rank。範囲の外なら空(端)。
func rankAt(list []ranked, i int) string {
	if i < 0 || i >= len(list) {
		return ""
	}
	return list[i].rank
}

// fillRanks は、rank のないワークスペース(list の先頭に並んでいる)だけに、表示の順を保つ rank を振り、
// 同じ保存に書く。list の rank も書き換える。rank を持つものは変えない。
func fillRanks(tx port.Tx, list []ranked) error {
	k := slices.IndexFunc(list, func(r ranked) bool { return r.rank != "" })
	if k < 0 { // rank を持つものがまだない: 先頭から順に伸ばす
		k = len(list)
		prev := ""
		for i := range list {
			list[i].rank = rank.After(prev)
			prev = list[i].rank
		}
	}
	for i := k - 1; i >= 0 && k < len(list); i-- { // 最初の rank より前に、後ろから詰める
		list[i].rank = rank.Between("", list[i+1].rank)
	}
	for _, r := range list[:k] {
		if err := tx.WriteWorkspaces().Append(r.ID, change.NewSet("rank", nil, r.rank)); err != nil {
			return err
		}
	}
	return nil
}
