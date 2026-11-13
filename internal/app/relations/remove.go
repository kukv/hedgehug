package relations

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// Remove は、self から見た label の関連を外す(02「関連」の「解除」。確認なし)。関連を持っているほうのログに
// remove を書く。持っていなければ何も保存しない。アクティビティは「{ID} と {ID} の {表記} を解除」。
// どちらかのアイテムがないか、知らない表記なら port.ErrNotFound。self が別の画面で削除された(root: true)
// アイテムなら、外すときに削除を取り消し、外す関連がなければ削除済みのまま port.ErrNotFound。
func (s Service) Remove(ctx context.Context, store port.StoreID, self item.ID, label relation.Label, other item.ID) error {
	typ, reverse, ok := relation.Parse(label)
	if !ok {
		return port.ErrNotFound
	}
	summary, err := s.summary(ctx, store, self, other, func(a, b string) string { return fmt.Sprintf("%s と %s の %s を解除", a, b, label) })
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		deleted, err := reviveSelf(ctx, tx, self)
		if err != nil {
			return err
		}
		snap, err := readSnapshot(tx)
		if err != nil {
			return err
		}
		owner, rel, ok := snap.holder(self, other, typ, reverse)
		if !ok {
			return port.Unchanged(deleted) // 行のない Write は何も残さない。削除済みのままなら取り消さず断る
		}
		return tx.WriteItems().Append(owner, change.NewRemove("relations", rel))
	})
}
