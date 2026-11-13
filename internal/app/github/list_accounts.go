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
	// Orgs は、GitHub App 未導入の組織名(記録の順)。認証エラーのときは空(D4b-13。記録は消さない)。
	Orgs []string
}

// ListAccounts は、登録したアカウントを登録した順に、状態を付けて返す。
type ListAccounts struct {
	Accounts port.Accounts
	Session  *Session
	Missing  port.AppMissing // GitHub App 未導入の組織の記録。nil なら出さない
}

func (u ListAccounts) Do(ctx context.Context) ([]AccountState, error) {
	accs, err := u.Accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	var missing map[account.ID][]port.MissingOrg
	if u.Missing != nil {
		if missing, err = u.Missing.List(ctx); err != nil {
			return nil, err
		}
	}
	out := make([]AccountState, 0, len(accs))
	for _, a := range accs {
		broken, reason := u.Session.state(a.ID)
		st := AccountState{Account: a, AuthError: broken, Reason: reason}
		if !broken {
			for _, m := range missing[a.ID] {
				st.Orgs = append(st.Orgs, m.Org)
			}
		}
		out = append(out, st)
	}
	return out, nil
}
