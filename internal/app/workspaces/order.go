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

// ranked は、並びの rank と旧い接頭辞を添えたワークスペース。rank が空なら、まだ並びを持たない(B2-7)。
type ranked struct {
	workspace.Workspace
	rank        string
	oldPrefixes []string
}

// orderedWorkspaces は、削除済みも含めたワークスペースを rank の順(同じなら内部 ID の順)で返す。rank のないものは先頭。
// 作成の rank が削除済みの rank を使い回さないために使う(B5b-4)。
func orderedWorkspaces(r port.Reader) ([]ranked, error) {
	list, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	return rankedInOrder(r, list)
}

// orderedLiveWorkspaces は、削除済みでないワークスペースを rank の順で返す(サイドバーの List と MoveWorkspace)。
func orderedLiveWorkspaces(r port.Reader) ([]ranked, error) {
	list, err := port.LiveWorkspaces(r)
	if err != nil {
		return nil, err
	}
	return rankedInOrder(r, list)
}

// rankedInOrder は、list に rank を添え、rank の順(同じなら内部 ID の順)に並べる。
func rankedInOrder(r port.Reader, list []workspace.Workspace) ([]ranked, error) {
	out := make([]ranked, 0, len(list))
	for _, w := range list {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		if err != nil {
			return nil, err
		}
		out = append(out, ranked{w, workspace.RankOf(log), workspace.OldPrefixes(log)})
	}
	slices.SortFunc(out, func(a, b ranked) int {
		return cmp.Or(cmp.Compare(a.rank, b.rank), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

// MoveWorkspace は、ワークスペースを 1 つ上(up)か下へ動かす(04「並び順と並べ替え」)。端なら何もしない。
// 位置が変わるのは動かした 1 件だけで、その rank を新しい隣どうしのあいだの値にする(B2-6)。
// 削除済みのワークスペースなら、削除を取り消してから動かす。端で動かないなら ErrNotFound(決定 R11)。
func (s Service) MoveWorkspace(ctx context.Context, store port.StoreID, ws workspace.ID, up bool) error {
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	var w workspace.Workspace
	if err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		w, err = port.EditableWorkspace(r, ws)
		return err
	}); err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("%s の並びを変更", oneLine(w.Name))}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		revived, err := port.ReviveWorkspace(ctx, tx, ws, "", "")
		if err != nil {
			return err
		}
		list, err := orderedLiveWorkspaces(tx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(list, func(r ranked) bool { return r.ID == ws })
		if i < 0 {
			return port.ErrNotFound
		}
		if (up && i == 0) || (!up && i == len(list)-1) {
			return unchanged(revived)
		}
		ranks, err := fillRanks(tx, list)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(ws, change.NewSet("rank", ranks[i], rank.Moved(ranks, i, up)))
	})
}

// fillRanks は、rank が空か、前のもの以下(同じ値を含む)のワークスペースだけに、表示の順を保つ rank を振り
// (rank.Refill)、同じ保存に書く。振ったあとの並びを返す。
func fillRanks(tx port.Tx, list []ranked) ([]string, error) {
	ranks := make([]string, len(list))
	for i, r := range list {
		ranks[i] = r.rank
	}
	out, changed := rank.Refill(ranks)
	for i, r := range list {
		if !changed[i] {
			continue
		}
		var from any
		if r.rank != "" {
			from = r.rank
		}
		if err := tx.WriteWorkspaces().Append(r.ID, change.NewSet("rank", from, out[i])); err != nil {
			return nil, err
		}
	}
	return out, nil
}
