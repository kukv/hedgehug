// Package items は、アイテムの操作(02)を持つ。1 ファイル 1 操作。
package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Service は、アイテムの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}

// withID は、アイテムと、その画面の ID(HH-12)を返す。アクティビティの 1 行に ID を書く操作が使う。
// アクティビティの 1 行は保存の前に決めるので、保存とは別に読む。
func (s Service) withID(ctx context.Context, store port.StoreID, id item.ID) (item.Item, string, error) {
	var it item.Item
	var display string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		if it, err = r.Items().Get(id); err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		display = it.DisplayID(w.Prefix)
		return nil
	})
	return it, display, err
}
