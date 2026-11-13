package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// GitHubTokens は、GitHub に問い合わせるトークンの口(D4a-2)。実装は app/github.Connector。
type GitHubTokens interface {
	// TokenFor は、Store に割り当てたアカウントのトークンとアカウント。割り当てがなければ NoAccount、
	// 認証エラーなら AuthError の *ghcall.Failure。別のアカウントでは読まない。
	TokenFor(ctx context.Context, store StoreID, repo string) (string, account.Account, error)
	// Rejected は、id のトークンが拒否された(401)ことを知らせ、認証エラーにする。
	Rejected(id account.ID)
	// Recheck は、認証エラーのアカウント(デバイスフローを除く)を確かめ直し、戻ったものがあれば true(D4a-18)。
	Recheck(ctx context.Context) (bool, error)
}
