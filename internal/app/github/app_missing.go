package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// RecheckOrgs は、アカウントの「再確認」(07「GitHub App 未導入」。D4b-14)。記録の組織ごとに、記録したリポジトリを
// 読み直す。
type RecheckOrgs struct {
	Accounts port.Accounts
	Conn     Connector // トークンと、認証エラーの状態(Conn.Session)
	GitHub   port.GitHub
	Missing  port.AppMissing
}

// RecheckResult は、読めるようになった組織(Cleared)と、まだ読めない組織(Still)。どちらも記録の順。
type RecheckResult struct{ Cleared, Still []string }

// Do は、読めた組織の記録を消し、App 未導入のままなら残す。トークンが拒否されたら認証エラーにして返す。
// ほかの失敗はすぐ返す(それまでに読めた組織は消したまま)。
func (u RecheckOrgs) Do(ctx context.Context, id account.ID) (RecheckResult, error) {
	res := RecheckResult{Cleared: []string{}, Still: []string{}}
	a, err := find(ctx, u.Accounts, id)
	if err != nil {
		return res, err
	}
	all, err := u.Missing.List(ctx)
	if err != nil {
		return res, err
	}
	orgs := all[a.ID]
	if len(orgs) == 0 {
		return res, nil
	}
	now := u.Conn.Clock.Now()
	tok, err := u.Conn.Token(ctx, a)
	if err != nil {
		return res, ghcall.Refuse(err, now)
	}
	for _, m := range orgs {
		err := u.GitHub.CheckRepo(ctx, tok, m.Repo)
		switch {
		case err == nil:
			if _, err := u.Missing.Clear(ctx, a.ID, m.Org); err != nil {
				return res, err
			}
			res.Cleared = append(res.Cleared, m.Org)
		case ghcall.Is(err, ghcall.AppNotInstalled):
			res.Still = append(res.Still, m.Org)
		case ghcall.Is(err, ghcall.TokenRejected):
			u.Conn.Rejected(a.ID)
			return res, ghcall.Refuse(authError(a), now)
		default:
			return res, ghcall.Refuse(err, now)
		}
	}
	return res, nil
}
