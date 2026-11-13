package githublink

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// Remove は、リンクを確認なしで外す(07「解除」)。PR は解除の記録に入る。ないリンクなら何も保存しない。
// アイテムがなければ port.ErrNotFound。外したリンクの取り直しの失敗の文言は消し、消したら EventGitHub を流す
// (文言は Store とリンクごとなので、同じ PR をリンクしている別のアイテムの行からも、次に読むまで消える)。
// 改名の前の名前で置いた文言は、ここでは消さない。
func (l Links) Remove(ctx context.Context, store port.StoreID, id item.ID, ref itemlink.Ref) error {
	v, err := l.editableView(ctx, store, id)
	if err != nil {
		return err
	}
	me, err := l.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: itemlink.RemoveSummary(v.display)}
	var removed *itemlink.Link
	err = l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		removed = nil
		before, err := readEditableItem(tx, id)
		if err != nil {
			return err
		}
		if err := port.ReviveItem(ctx, tx, id); err != nil { // 別の画面で削除されていたら、最初に取り消す(決定 S10)
			return err
		}
		cur, err := readItem(tx, id)
		if err != nil {
			return err
		}
		link, ok := cur.links.Find(ref)
		if !ok || link.Kind != ref.Kind {
			return port.Unchanged(before.item.Deleted) // 削除済みのまま外すものがなければ、取り消さず断る
		}
		removed = &link
		return tx.WriteItems().Append(id, cur.links.Detach(link)...)
	})
	if err != nil || removed == nil {
		return err
	}
	changed, err := l.setFailure(ctx, store, removed.Key(), "")
	if changed {
		l.Events.Publish(port.Event{Kind: port.EventGitHub, Store: store})
	}
	return err
}
