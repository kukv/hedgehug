package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// DeleteAccount は、アカウントをすぐに削除する(07「削除の影響」)。割り当てた Store は「なし」になる。
// デバイスフローのトークンはキーチェーンから消す(この小計画の決定 D2-12)。始まっている処理は、取り出し済みの
// トークンで最後まで続く(トークンは Connector の外では、その処理のあいだだけ持つ)。
type DeleteAccount struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	Stores   port.StoreGitHub
}

func (u DeleteAccount) Do(ctx context.Context, id account.ID) error {
	a, err := find(ctx, u.Accounts, id)
	if err != nil {
		return err
	}
	// 先に割り当てを外す。一覧から外したあとで失敗すると、どの Store にも見えない割り当てが残るため。
	if err := u.Stores.Unassign(ctx, id); err != nil {
		return err
	}
	if err := u.Accounts.Remove(ctx, id); err != nil {
		return err
	}
	u.Session.forget(id)
	if a.Method == account.DeviceFlow {
		// アカウントはもう消えている。キーチェーンの項目を消せなくても、削除は済んだものとして返す
		// (エラーにすると、やり直しは 404 になるだけなので)。
		_ = u.Secrets.DeleteDeviceToken(ctx, a.Login)
	}
	return nil
}

// Impact は、削除と編集の確認の本文を組む(07「削除の影響」「編集」、この小計画の決定 D2-10)。
type Impact struct {
	Accounts port.Accounts
	Stores   port.StoreGitHub
}

// Delete は、「アカウントを削除」の確認の本文の行。
func (u Impact) Delete(ctx context.Context, id account.ID) ([]string, error) {
	a, uses, err := u.uses(ctx, id)
	if err != nil {
		return nil, err
	}
	login, err := u.Stores.MachineLogin(ctx)
	if err != nil {
		return nil, err
	}
	return account.DeleteLines(a, uses, login), nil
}

// Edit は、ログイン名を login に変えるときの「アカウントを変更」の確認の本文の行。空なら確認を出さない。
func (u Impact) Edit(ctx context.Context, id account.ID, login string) ([]string, error) {
	a, uses, err := u.uses(ctx, id)
	if err != nil {
		return nil, err
	}
	return account.EditLines(a, login, uses), nil
}

func (u Impact) uses(ctx context.Context, id account.ID) (account.Account, []account.StoreUse, error) {
	a, err := find(ctx, u.Accounts, id)
	if err != nil {
		return account.Account{}, nil, err
	}
	stores, err := u.Stores.List(ctx)
	if err != nil {
		return account.Account{}, nil, err
	}
	var uses []account.StoreUse
	for _, s := range stores {
		if s.Account == id {
			uses = append(uses, account.StoreUse{Name: s.Name, Remote: s.Remote})
		}
	}
	return a, uses, nil
}
