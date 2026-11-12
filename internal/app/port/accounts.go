package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Accounts は、このマシンに登録したアカウントの一覧の口(01「マシンごとに持つ」)。
type Accounts interface {
	List(ctx context.Context) ([]account.Account, error) // 登録した順
	Add(ctx context.Context, a account.Account) error
}
