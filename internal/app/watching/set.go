package watching

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Set は、あなたがアイテムをウォッチ中かどうかを on にそろえる(02「ウォッチ」)。すでにその状態なら何も保存しない。
// アクティビティは「{ID} のウォッチを開始」「{ID} のウォッチを解除」。アイテムがなければ port.ErrNotFound。
func (s Service) Set(ctx context.Context, store port.StoreID, id item.ID, on bool) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	shown, err := s.displayOf(ctx, store, id)
	if err != nil {
		return err
	}
	verb := "解除"
	if on {
		verb = "開始"
	}
	op := port.Operation{Actor: change.User(me), Summary: shown + " のウォッチを" + verb}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		return tx.WriteItems().Append(id, drafts(it.Watchers, me, on)...)
	})
}

// drafts は、あなたのウォッチ中の状態を on にそろえる行。ログイン名は大文字と小文字を区別せずに比べ(01)、
// 取り除くときは保存してある綴りで書く(複数の値の取り除きは、値が同じでないと効かない)。
func drafts(watchers []string, me string, on bool) []change.Draft {
	var stored []string
	for _, w := range watchers {
		if strings.EqualFold(w, me) {
			stored = append(stored, w)
		}
	}
	if on {
		if len(stored) > 0 {
			return nil
		}
		return []change.Draft{change.NewAdd("watchers", me)}
	}
	out := make([]change.Draft, 0, len(stored))
	for _, w := range stored {
		out = append(out, change.NewRemove("watchers", w))
	}
	return out
}
