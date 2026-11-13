package items

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Rename は、タイトルを変える(02「タイトル」)。整えたあとが空なら断り、前と同じなら何も保存しない。
// どちらでもなければ 1 回の保存で、アクティビティは「{ID} のタイトルを変更」。変えたあとのアイテムを返す。
func (s Service) Rename(ctx context.Context, store port.StoreID, id item.ID, title string) (item.Item, error) {
	title = item.NormalizeTitle(title)
	if title == "" {
		return item.Item{}, refusal.New("タイトルは空にできません")
	}
	_, display, err := s.withID(ctx, store, id)
	if err != nil {
		return item.Item{}, err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return item.Item{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s のタイトルを変更", display)}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := port.LiveItem(tx, id)
		if err != nil {
			return err
		}
		if it.Title == title {
			return nil // 行のない Write は何も残さない
		}
		return tx.WriteItems().Append(id, change.NewSet("title", it.Title, title))
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, store, id)
}
