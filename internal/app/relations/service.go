// Package relations は、アイテムの関連(02「関連」)の操作を持つ。1 ファイル 1 操作。
package relations

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Service は、関連の操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
}

// summary は、アクティビティの 1 行を保存の前に決める。2 つのアイテムの画面の ID(HH-1)が要る。
// どちらかがなければ port.ErrNotFound。
func (s Service) summary(ctx context.Context, store port.StoreID, self, other item.ID, text func(a, b string) string) (string, error) {
	var out string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		snap, err := readSnapshot(r)
		if err != nil {
			return err
		}
		// 自分は、別の画面で削除された(root: true)アイテムでもよい(保存の中で削除を取り消す)。相手は生きているもの
		a, err := port.EditableItem(r, self)
		if err != nil {
			return err
		}
		b, okB := snap.byID[other]
		if !okB {
			return port.ErrNotFound
		}
		out = text(snap.display(a), snap.display(b))
		return nil
	})
	return out, err
}

// reviveSelf は、保存の中で、操作したアイテム self が別の画面で削除された(root: true)ものなら、削除を取り消す。
// 取り消す前の削除の印を返す(何も書かずに返す分かれ道で port.Unchanged に渡す。決定 S10)。
// 親と一緒に削除された・祖先かプロジェクトが削除済みなら port.ErrNotFound。
func reviveSelf(ctx context.Context, tx port.Tx, self item.ID) (*change.Deletion, error) {
	it, err := port.EditableItem(tx, self)
	if err != nil {
		return nil, err
	}
	return it.Deleted, port.ReviveItem(ctx, tx, self)
}
