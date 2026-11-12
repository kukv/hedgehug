package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Secrets はトークンの口。トークンは、この口の外では、問い合わせに渡すあいだだけ持つ。
type Secrets interface {
	// Token は、アカウントの認証方法でトークンを読む。読めなければ *ghcall.Failure
	// (GhTokenMissing・GhTimeout・EnvMissing・KeychainMissing)を返す。
	Token(ctx context.Context, a account.Account) (string, error)
}
