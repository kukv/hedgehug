package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// EditAccount は、アカウントの値を入れ替える(07「編集」)。新しい値で認証の確認をして、確かめられたときだけ保存する。
// 新しい認証方法がデバイスフローなら、サインイン(SignIn)で保存する。Store の割り当ては ID で持つので変わらない。
type EditAccount struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Clock    port.Clock
}

type EditAccountInput struct {
	ID     account.ID
	Name   string
	Login  string
	Method account.Method
}

func (u EditAccount) Do(ctx context.Context, in EditAccountInput) (account.Account, error) {
	a, fp, err := u.verify(ctx, in)
	if err != nil {
		return account.Account{}, ghcall.Refuse(err, u.Clock.Now())
	}
	if err := u.Accounts.Update(ctx, a); err != nil {
		return account.Account{}, err
	}
	u.Session.markOK(a.ID, fp)
	return a, nil
}

func (u EditAccount) verify(ctx context.Context, in EditAccountInput) (account.Account, [32]byte, error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return account.Account{}, [32]byte{}, err
	}
	if _, err := find(ctx, u.Accounts, in.ID); err != nil {
		return account.Account{}, [32]byte{}, err
	}
	a, err := account.Check(account.Account{Name: in.Name, Login: in.Login, Method: in.Method}, existing, in.ID)
	if err != nil {
		return account.Account{}, [32]byte{}, err
	}
	a.ID = in.ID
	if a.Method == account.DeviceFlow {
		return account.Account{}, [32]byte{}, ErrDeviceFlow
	}
	fp, err := verifyToken(ctx, u.Secrets, u.GitHub, a)
	return a, fp, err
}
