package port

import "context"

// SyncTokens は、Store に割り当てたアカウントのトークンを得る口(07「接続のたびの照合」)。
// 実装は app/github の Connector(読み直し・照合・取り直し・認証エラーへの変更を 1 か所で行う)。
type SyncTokens interface {
	// StoreToken は、Store のアカウントのトークン。割り当てがなければ NoAccount、認証エラーなら AuthError、
	// 読めなければ GhTokenMissing などの *ghcall.Failure を返す。
	StoreToken(ctx context.Context, store StoreID) (string, error)
	// StoreRejected は、Store のアカウントのトークンを remote が拒否した(401)ことを知らせ、認証エラーにする
	// (07「状態」)。割り当てがなければ何もしない。
	StoreRejected(ctx context.Context, store StoreID)
}
