package descriptions

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// SaveItem は、アイテムの説明を保存する(02「説明」)。base は編集を始めたときの説明、text は入力。
// アクティビティは「{ID} の説明を変更」、履歴には desc の行が 1 つ残る。
func (s Service) SaveItem(ctx context.Context, store port.StoreID, id item.ID, base, text string) (Result, error) {
	display, err := s.displayID(ctx, store, id)
	if err != nil {
		return Result{}, err
	}
	return s.save(ctx, store, display+" の説明を変更", base, text, slot{
		read: func(r port.Reader) (string, error) { return r.Items().Description(id) },
		write: func(tx port.Tx, merged string) error {
			return errors.Join(tx.WriteItems().Describe(id, merged), tx.WriteItems().Append(id, change.NewDesc()))
		},
	})
}

// displayID は、アクティビティの 1 行に書く、アイテムの画面の ID(HH-12)。保存の前に決めるので別に読む。
func (s Service) displayID(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var display string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.LiveItem(r, id)
		if err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		display = it.DisplayID(w.Prefix)
		return nil
	})
	return display, err
}
