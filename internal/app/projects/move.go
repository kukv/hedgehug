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
		if err := fillProjectRanks(tx, list); err != nil {
			return nil, err
		}
		return []change.Draft{change.NewSet("rank", list[i].Rank, movedRank(list, i, up))}, nil
	})
	return err
}

// movedRank は、list[i] を 1 つ上(up)か下へ動かしたときの rank。新しい隣どうしのあいだの値。
func movedRank(list []project.Project, i int, up bool) string {
	if up {
		return rank.Between(rankAt(list, i-2), list[i-1].Rank)
	}
	return rank.Between(list[i+1].Rank, rankAt(list, i+2))
}

// rankAt は list[i] の rank。範囲の外なら空(端)。
func rankAt(list []project.Project, i int) string {
	if i < 0 || i >= len(list) {
		return ""
	}
	return list[i].Rank
}

// fillProjectRanks は、rank が空か、前のもの以下(同じ値を含む)のプロジェクトだけに、表示の順を保つ rank を振り、
// 同じ保存に書く。list の Rank も書き換える。空でなく厳密に増えている rank は変えない。
// 変えるものの rank は、前に残るものと、次に残るもののあいだの値にする。
func fillProjectRanks(tx port.Tx, list []project.Project) error {
	keep := make([]bool, len(list))
	prev := ""
	for i, p := range list {
		if p.Rank != "" && p.Rank > prev {
			keep[i], prev = true, p.Rank
		}
	}
	prev = ""
	for i := range list {
		if keep[i] {
			prev = list[i].Rank
			continue
		}
		var from any
		if list[i].Rank != "" {
			from = list[i].Rank
		}
		list[i].Rank = rank.Between(prev, nextKept(list, keep, i))
		prev = list[i].Rank
		if err := tx.WriteProjects().Append(list[i].ID, change.NewSet("rank", from, list[i].Rank)); err != nil {
			return err
		}
	}
	return nil
}

// nextKept は、list[i] より後ろで最初に残るものの rank。なければ空。
func nextKept(list []project.Project, keep []bool, i int) string {
	for j := i + 1; j < len(list); j++ {
		if keep[j] {
			return list[j].Rank
		}
	}
	return ""
}
