package githublink

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// Remove は、リンクを確認なしで外す(07「解除」)。PR は解除の記録に入る。ないリンクなら何も保存しない。
// アイテムがなければ port.ErrNotFound。
func (l Links) Remove(ctx context.Context, store port.StoreID, id item.ID, ref itemlink.Ref) error {
	v, err := l.view(ctx, store, id)
	if err != nil {
		return err
	}
	me, err := l.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: itemlink.RemoveSummary(v.display)}
	return l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := readItem(tx, id)
		if err != nil {
			return err
		}
		link, ok := cur.links.Find(ref)
		if !ok || link.Kind != ref.Kind {
			return nil
		}
		return tx.WriteItems().Append(id, cur.links.Detach(link)...)
	})
}
