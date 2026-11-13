// Package watching は、アイテムのウォッチの操作(02「ウォッチ」)を持つ。
package watching

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Service は、ウォッチの操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
}

// displayOf は、アクティビティの 1 行に書くアイテムの ID(HH-12)。保存の前に読む。別の画面で削除された(root: true)アイテムも読める。
func (s Service) displayOf(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var shown string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.EditableItem(r, id)
		if err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		shown = it.DisplayID(w.Prefix)
		return nil
	})
	return shown, err
}
