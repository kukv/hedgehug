package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// DeleteAccount は、アカウントをすぐに削除する(07「削除の影響」)。割り当てた Store は「なし」になる。
// デバイスフローのトークンはキーチェーンから消す(この小計画の決定 D2-12)。消せなくても削除は成功として返す。
// 始まっている処理は、取り出し済みのトークンで最後まで続く(トークンは Connector の外では、その処理のあいだだけ持つ)。
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
		_ = u.Secrets.DeleteDeviceToken(ctx, a.Login)
	}
	return nil
}
