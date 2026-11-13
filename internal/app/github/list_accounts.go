package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// AccountState は、一覧の 1 行(07「状態」)。
type AccountState struct {
	Account   account.Account
	AuthError bool
	Reason    string // 「認証エラー」の下に出す赤字(ログイン名の不一致)。なければ空
}

// ListAccounts は、登録したアカウントを登録した順に、状態を付けて返す。
type ListAccounts struct {
	Accounts port.Accounts
	Session  *Session
}

func (u ListAccounts) Do(ctx context.Context) ([]AccountState, error) {
	accs, err := u.Accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AccountState, 0, len(accs))
	for _, a := range accs {
		broken, reason := u.Session.state(a.ID)
		out = append(out, AccountState{Account: a, AuthError: broken, Reason: reason})
	}
	return out, nil
}
