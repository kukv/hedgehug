package githublink

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Pass は、外のテストが 1 回の読み取りを作るための pass。
type Pass = pass

// NewPass は、新しい読み取り。
func NewPass() *Pass { return newPass() }

// AutoLink は、Store の削除済みでないアイテムを読んでから autoLink を呼ぶ(テストの口)。
func (l Links) AutoLink(ctx context.Context, store port.StoreID, repo string, pull port.Pull, first bool, p *Pass) (bool, error) {
	items, err := l.readStoreItems(ctx, store)
	if err != nil {
		return false, err
	}
	return l.autoLink(ctx, store, repo, repo, pull, items, first, p)
}
