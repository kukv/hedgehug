package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// Reauth は、gh CLI・環境変数・OS のキーチェーンのアカウントの認証の確認をやり直す(07「再認証」)。
// デバイスフローはサインイン(SignIn)でやり直す。失敗しても状態は変えない。
type Reauth struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Clock    port.Clock
}

func (u Reauth) Do(ctx context.Context, id account.ID) (account.Account, error) {
	a, err := find(ctx, u.Accounts, id)
	if err != nil {
		return account.Account{}, err
	}
	if a.Method == account.DeviceFlow {
		return account.Account{}, ErrDeviceFlow
	}
	fp, err := verifyToken(ctx, u.Secrets, u.GitHub, a)
	if err != nil {
		return account.Account{}, ghcall.Refuse(err, u.Clock.Now())
	}
	u.Session.markOK(a.ID, fp)
	return a, nil
}

// find は、ID のアカウント。なければ port.ErrNotFound。
func find(ctx context.Context, accounts port.Accounts, id account.ID) (account.Account, error) {
	accs, err := accounts.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	for _, a := range accs {
		if a.ID == id {
			return a, nil
		}
	}
	return account.Account{}, port.ErrNotFound
}
