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
	prev, a, fp, err := u.verify(ctx, in)
	if err != nil {
		return account.Account{}, ghcall.Refuse(err, u.Clock.Now())
	}
	if err := u.Accounts.Update(ctx, a); err != nil {
		return account.Account{}, err
	}
	u.dropOldDeviceToken(ctx, prev, a)
	u.Session.markOK(a.ID, fp)
	return a, nil
}

// dropOldDeviceToken は、デバイスフローから替えて誰も読まなくなったキーチェーンのトークンを消す(決定 D2-13)。
// 残すのは、同じログイン名の OS のキーチェーンへ替えたときだけ(キーチェーンの名前は大文字と小文字を区別する)。
// 消せなくても編集は失敗にしない。
func (u EditAccount) dropOldDeviceToken(ctx context.Context, prev, a account.Account) {
	if prev.Method != account.DeviceFlow || (a.Method == account.Keychain && a.Login == prev.Login) {
		return
	}
	_ = u.Secrets.DeleteDeviceToken(ctx, prev.Login)
}

// verify は、編集前のアカウント(prev)、保存する値、確かめたトークンの指紋を返す。
func (u EditAccount) verify(ctx context.Context, in EditAccountInput) (prev, a account.Account, fp [32]byte, err error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return prev, a, fp, err
	}
	if prev, err = find(ctx, u.Accounts, in.ID); err != nil {
		return prev, a, fp, err
	}
	a, err = account.Check(account.Account{Name: in.Name, Login: in.Login, Method: in.Method}, existing, in.ID)
	if err != nil {
		return prev, a, fp, err
	}
	a.ID = in.ID
	if a.Method == account.DeviceFlow {
		return prev, a, fp, ErrDeviceFlow
	}
	fp, err = verifyToken(ctx, u.Secrets, u.GitHub, a)
	return prev, a, fp, err
}
