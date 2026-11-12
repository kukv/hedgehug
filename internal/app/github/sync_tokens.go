package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// StoreToken は、Store の同期に使うトークン(port.SyncTokens。決定 C2-3)。読み直し・照合・取り直し・
// 認証エラーは TokenFor と同じ。
func (c Connector) StoreToken(ctx context.Context, store port.StoreID) (string, error) {
	tok, _, err := c.TokenFor(ctx, store, "")
	return tok, err
}

// StoreRejected は、Store に割り当てたアカウントのトークンを remote が拒否した(401)ことを受け、
// そのアカウントを認証エラーにする(07「状態」)。割り当てがなければ何もしない。
func (c Connector) StoreRejected(ctx context.Context, store port.StoreID) {
	a, err := c.storeAccount(ctx, store)
	if err != nil || a.ID == "" {
		return
	}
	c.Rejected(a.ID)
}
