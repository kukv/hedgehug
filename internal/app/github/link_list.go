package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// List は、アイテムのリンクを、リンクした順に、このマシンが読んだチェックの結果とともに返す(07「表示と更新」)。
// GitHub には問い合わせない。アイテムがなければ port.ErrNotFound。
func (l Links) List(ctx context.Context, store port.StoreID, id item.ID) ([]LinkRow, error) {
	v, err := l.view(ctx, store, id)
	if err != nil {
		return nil, err
	}
	checks, err := l.Checks.Read(ctx, store)
	if err != nil {
		return nil, err
	}
	return rowsOf(v.links.Links, checks, nil), nil
}
