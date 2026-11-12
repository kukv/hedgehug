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
	cur, a, fp, err := u.verify(ctx, in)
	if err != nil {
		return account.Account{}, ghcall.Refuse(err, u.Clock.Now())
	}
	if err := u.Accounts.Update(ctx, a); err != nil {
		return account.Account{}, err
	}
	u.Session.markOK(a.ID, fp)
	// デバイスフローから替えたとき、古い項目を読むのは、同じログイン名の OS のキーチェーンの認証方法だけ(決定 D2-5)。
	// ほかは誰も読まないトークンが残るので消す(決定 D2-12・D2-13)。消せなくても編集は済んでいる。
	if cur.Method == account.DeviceFlow && (a.Method != account.Keychain || a.Login != cur.Login) {
		_ = u.Secrets.DeleteDeviceToken(ctx, cur.Login)
	}
	return a, nil
}

// verify は、いまの値(cur)と、確かめた新しい値(a)と、そのトークンの指紋を返す。
func (u EditAccount) verify(ctx context.Context, in EditAccountInput) (cur, a account.Account, fp [32]byte, err error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return cur, a, fp, err
	}
	if cur, err = find(ctx, u.Accounts, in.ID); err != nil {
		return cur, a, fp, err
	}
	if a, err = account.Check(account.Account{Name: in.Name, Login: in.Login, Method: in.Method}, existing, in.ID); err != nil {
		return cur, a, fp, err
	}
	a.ID = in.ID
	if a.Method == account.DeviceFlow {
		return cur, a, fp, ErrDeviceFlow
	}
	fp, err = verifyToken(ctx, u.Secrets, u.GitHub, a)
	return cur, a, fp, err
}
