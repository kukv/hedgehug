package watching

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Set は、あなたがアイテムをウォッチ中かどうかを on にそろえる(02「ウォッチ」)。すでにその状態なら何も保存しない。
// 別の画面で削除された(root: true)アイテムは、行を書くときだけ削除を取り消し、行がなければ port.ErrNotFound で断る。
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
		editable, err := port.EditableItem(tx, id)
		if err != nil {
			return err
		}
		if err := port.ReviveItem(ctx, tx, id); err != nil {
			return err
		}
		it, err := port.LiveItem(tx, id)
		if err != nil {
			return err
		}
		ds := drafts(it.Watchers, me, on)
		if len(ds) == 0 {
			return port.Unchanged(editable.Deleted) // 行のない Write は何も残さない
		}
		return tx.WriteItems().Append(id, ds...)
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
