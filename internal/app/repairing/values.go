package repairing

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// restoreReferenced は、設定が指している値(初期ステータス・分類の既定・初期優先度)や、
// 0 個になる分類のステータスの削除を取り消す(決定 C5b2-3・C5b2-4)。
// 削除の op ごとに 1 回保存し、その op の行があるログごとに change.Undo の行を書く。
func restoreReferenced(ctx context.Context, w port.RepairWriter) error {
	s, err := readSnapshot(w)
	if err != nil {
		return err
	}
	for _, f := range valueRestores(s, w.Facts()) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := undo(w, s, f); err != nil {
			return err
		}
	}
	return nil
}

// valueRestores は、すべてのワークスペースの取り消す値の削除を、削除の op の辞書順に返す。
// 同じ op は、最初に見つけたログのもの 1 つ。
func valueRestores(s snapshot, facts repair.Facts) []found {
	byOp := map[change.ID]found{}
	for i, e := range s.entities {
		if e.ref.Kind != change.KindWorkspace {
			continue
		}
		for _, o := range repair.ValueRestores(e.log, facts) {
			if _, ok := byOp[o.Op]; !ok {
				byOp[o.Op] = found{Overlap: o, at: i}
			}
		}
	}
	return sortedByOp(byOp)
}
