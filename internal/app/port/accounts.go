package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Accounts は、このマシンに登録したアカウントの一覧の口(01「マシンごとに持つ」)。
type Accounts interface {
	List(ctx context.Context) ([]account.Account, error) // 登録した順
	Add(ctx context.Context, a account.Account) error
	// Update は、ID が同じアカウントの値を入れ替える(並びは変えない)。なければ ErrNotFound。
	Update(ctx context.Context, a account.Account) error
	// Remove は、アカウントを一覧から外す。なければ ErrNotFound。
	Remove(ctx context.Context, id account.ID) error
}
