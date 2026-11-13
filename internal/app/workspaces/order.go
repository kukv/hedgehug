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
		list, err := orderedLiveWorkspaces(tx)
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

// fillRanks は、rank が空か、前のもの以下(同じ値を含む)のワークスペースだけに、表示の順を保つ rank を振り、
// 同じ保存に書く。list の rank も書き換える。空でなく厳密に増えている rank は変えない。
// 変えるものの rank は、前に残るものと、次に残るもののあいだの値にする。
func fillRanks(tx port.Tx, list []ranked) error {
	keep := keptRanks(list)
	prev := ""
	for i := range list {
		if keep[i] {
			prev = list[i].rank
			continue
		}
		var from any
		if list[i].rank != "" {
			from = list[i].rank
		}
		list[i].rank = rank.Between(prev, nextKept(list, keep, i))
		prev = list[i].rank
		if err := tx.WriteWorkspaces().Append(list[i].ID, change.NewSet("rank", from, list[i].rank)); err != nil {
			return err
		}
	}
	return nil
}

// keptRanks は、rank を変えないもの(空でなく、前に残るものより大きい)を true にして返す。
func keptRanks(list []ranked) []bool {
	keep := make([]bool, len(list))
	prev := ""
	for i, r := range list {
		if r.rank != "" && r.rank > prev {
			keep[i], prev = true, r.rank
		}
	}
	return keep
}

// nextKept は、list[i] より後ろで最初に残るものの rank。なければ空。
func nextKept(list []ranked, keep []bool, i int) string {
	for j := i + 1; j < len(list); j++ {
		if keep[j] {
			return list[j].rank
		}
	}
	return ""
}
