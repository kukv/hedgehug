package githublink

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// List は、アイテムのリンクを、リンクした順に、このマシンが読んだチェックの結果と取り直しの失敗の文言とともに返す(07「表示と更新」)。
// GitHub には問い合わせない。アイテムがなければ port.ErrNotFound。取り直さない Store(閲覧のみ・このマシンに
// アカウントがない。D3-9)では、Refresh と同じく、置いてある失敗の文言を出さない(消しもしない)。
func (l Links) List(ctx context.Context, store port.StoreID, id item.ID) ([]LinkRow, error) {
	skip, err := l.skipRefresh(ctx, store)
	if err != nil {
		return nil, err
	}
	return l.rows(ctx, store, id, skip)
}

// rows は、アイテムのリンクの行。hideErrors なら、置いてある取り直しの失敗の文言を添えない。
func (l Links) rows(ctx context.Context, store port.StoreID, id item.ID, hideErrors bool) ([]LinkRow, error) {
	v, err := l.view(ctx, store, id)
	if err != nil {
		return nil, err
	}
	checks, err := l.Checks.Read(ctx, store)
	if err != nil {
		return nil, err
	}
	failures := map[string]string{}
	if !hideErrors {
		if failures, err = l.Reads.Failures(ctx, store); err != nil {
			return nil, err
		}
	}
	return rowsOf(v.links.Links, checks, failures), nil
}
