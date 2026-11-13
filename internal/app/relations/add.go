package relations

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// Add は、self から見て、other とのあいだに label の関連を足す(02「関連」の「追加」)。たとえば self が
// 「blocked by HH-3」を足すと、HH-3 のログに「このアイテムをブロックする」関連を書く。アクティビティは
// 「{ID} {表記} {ID}」。
// 相手が自分・Step・ないアイテム、知らない表記なら port.ErrNotFound。同じ種類の関連がある(向きが逆でも)か、
// blocks で循環になるときは断る。条件は保存の中で読み直して確かめる。self が別の画面で削除された(root: true)
// アイテムなら、関連を足すときに self の削除を取り消す(相手が削除済みなら足さずに断る)。
func (s Service) Add(ctx context.Context, store port.StoreID, self item.ID, label relation.Label, other item.ID) error {
	typ, reverse, ok := relation.Parse(label)
	if !ok {
		return port.ErrNotFound
	}
	summary, err := s.summary(ctx, store, self, other, func(a, b string) string { return fmt.Sprintf("%s %s %s", a, label, b) })
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		if _, err := reviveSelf(ctx, tx, self); err != nil {
			return err
		}
		snap, err := readSnapshot(tx)
		if err != nil {
			return err
		}
		o, okO := snap.byID[other]
		if _, okS := snap.byID[self]; !okS || !okO || self == other || o.Kind == item.Step {
			return port.ErrNotFound
		}
		if reason := snap.reason(self, other, label, typ, reverse); reason != "" {
			return refusal.New(reason)
		}
		owner, rel := self, item.Relation{Type: typ, To: other}
		if reverse {
			owner, rel = other, item.Relation{Type: typ, To: self}
		}
		return tx.WriteItems().Append(owner, change.NewAdd("relations", rel))
	})
}
