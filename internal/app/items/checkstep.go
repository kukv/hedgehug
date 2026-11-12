package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// CheckStep は、Step のチェックを入り切りする(02「Step のチェック」)。done なら完了の既定、そうでなければ
// 未着手の既定のステータスにする(03「分類ごとの既定のステータス」)。保存とアクティビティは SetProperty と同じ。
// 遷移の制約は後半(A3b)で SetProperty の中に足す(決定 2)。
func (s Service) CheckStep(ctx context.Context, store port.StoreID, id item.ID, done bool) (item.Item, error) {
	ec, err := s.editContext(ctx, store, id)
	if err != nil {
		return item.Item{}, err
	}
	if ec.item.Kind != item.Step {
		return item.Item{}, port.ErrNotFound
	}
	cat := workspace.CategoryTodo
	if done {
		cat = workspace.CategoryDone
	}
	return s.SetProperty(ctx, store, id, "status", change.Value(ec.settings.CategoryDefaults[cat]))
}
