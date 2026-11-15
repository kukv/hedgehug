package views

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// KeyImpact は、ワークスペース (store, ws) の追加プロパティ key を削除すると、保存したビューの条件が使えなくなるか(E3b-18)。
// 別のワークスペースにも同じキーがあるか、どのビューの条件にもそのキーがなければ false。
func (s *Service) KeyImpact(ctx context.Context, store port.StoreID, ws workspace.ID, key string) (bool, error) {
	held, err := s.HeldElsewhere(ctx, store, ws, key)
	if err != nil || held {
		return false, err
	}
	list, err := s.Store.List(ctx)
	if err != nil {
		return false, err
	}
	for _, v := range list {
		if query.UsesKey(v.Content.Query, key) {
			return true, nil
		}
	}
	return false, nil
}
