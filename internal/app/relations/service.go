// Package relations は、アイテムの関連(02「関連」)の操作を持つ。1 ファイル 1 操作。
package relations

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
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
		a, okA := snap.byID[self]
		b, okB := snap.byID[other]
		if !okA || !okB {
			return port.ErrNotFound
		}
		out = text(snap.display(a), snap.display(b))
		return nil
	})
	return out, err
}
