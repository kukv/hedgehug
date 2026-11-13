// Package repairing は、同期で合わせたあと、送る前に行う修復(保存形式「同期の修復」)を書く。
// gitstore の Replicator が、Store のロックを持ったまま Run を呼ぶ(決定 C3a-2・C3a-3)。
package repairing

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// step は、修復の段 1 つ。直すものごとに 1 回ずつ w.Write で保存する。
type step func(ctx context.Context, w port.RepairWriter) error

// steps は、修復の順(決定 C3a-5)。(1) は gitstore が Run の前に直す。
var steps = []step{
	restoreOverlaps, // 削除と変更の重なり
}

// syncActor は、修復の保存の実行者。
var syncActor = change.Actor{Kind: change.ActorSync}

// Run は、修復の段を順に行う。port.RepairFunc として gitstore の Replicator に置く。
func Run(ctx context.Context, w port.RepairWriter) error {
	for _, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s(ctx, w); err != nil {
			return err
		}
	}
	return nil
}
