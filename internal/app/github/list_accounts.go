package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// ListAccounts は、登録したアカウントを登録した順に返す。
type ListAccounts struct {
	Accounts port.Accounts
}

func (u ListAccounts) Do(ctx context.Context) ([]account.Account, error) {
	return u.Accounts.List(ctx)
}
