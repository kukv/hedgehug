// Package github は、GitHub 連携の usecase(仕様 07)を持つ。
package github

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// ErrDeviceFlow は、デバイスフローのアカウントをこの操作で登録しようとしたとき(D2 のサインインで登録する)。
var ErrDeviceFlow = errors.New("a device flow account is added by signing in")

// AddAccount は、アカウントを登録する(07「登録」「認証の確認」)。GitHub で本人を確かめられたときだけ登録する。
// Store を変えないので、保存もアクティビティも作らない。
type AddAccount struct {
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	IDs      port.IDs
	Clock    port.Clock
}

type AddAccountInput struct {
	Name   string
	Login  string
	Method account.Method
}

func (u AddAccount) Do(ctx context.Context, in AddAccountInput) (account.Account, error) {
	a, err := u.verify(ctx, in)
	if err != nil {
		return account.Account{}, ghcall.Refuse(err, u.Clock.Now())
	}
	a.ID = account.ID(u.IDs.New())
	if err := u.Accounts.Add(ctx, a); err != nil {
		return account.Account{}, err
	}
	return a, nil
}

// verify は、入力を確かめ、トークンを読み、GitHub でログイン名を照らす。
func (u AddAccount) verify(ctx context.Context, in AddAccountInput) (account.Account, error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	a, err := account.Check(account.Account{Name: in.Name, Login: in.Login, Method: in.Method}, existing, "")
	if err != nil {
		return account.Account{}, err
	}
	if a.Method == account.DeviceFlow {
		return account.Account{}, ErrDeviceFlow
	}
	token, err := u.Secrets.Token(ctx, a)
	if err != nil {
		return account.Account{}, err
	}
	login, err := u.GitHub.Viewer(ctx, token)
	if err != nil {
		return account.Account{}, err
	}
	if !strings.EqualFold(login, a.Login) {
		return account.Account{}, &ghcall.Failure{Kind: ghcall.LoginMismatch, Login: login}
	}
	return a, nil
}
