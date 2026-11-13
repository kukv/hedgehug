package github

import (
	"context"
	"time"

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

// Do は、読めた組織の記録を消し、App 未導入のままなら残す。記録したリポジトリがただ読めない(RepoUnreadable:
// 打ち間違い・削除)なら、App はもう妨げていないので、読めたときと同じに消す。トークンが拒否されたら認証エラーに
// してすぐ返す。ほかの失敗(接続できない・利用上限など)は、その組織を Still に入れて残りの組織を続け、最後に
// 1 つ目の失敗を返す(それまでに読めた組織は消したまま。画面は断りを受けたら一覧を読み直す)。
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
	return u.checkAll(ctx, a, tok, orgs, now)
}

// checkAll は、記録の組織 orgs を 1 つずつ読み直す(Do)。
func (u RecheckOrgs) checkAll(ctx context.Context, a account.Account, tok string, orgs []port.MissingOrg, now time.Time) (RecheckResult, error) {
	res := RecheckResult{Cleared: []string{}, Still: []string{}}
	var first error
	for _, m := range orgs {
		err := u.GitHub.CheckRepo(ctx, tok, m.Repo)
		switch {
		case err == nil || ghcall.Is(err, ghcall.RepoUnreadable):
			if _, err := u.Missing.Clear(ctx, a.ID, m.Org, m.Repo); err != nil {
				return res, err
			}
			res.Cleared = append(res.Cleared, m.Org)
		case ghcall.Is(err, ghcall.TokenRejected):
			u.Conn.Rejected(a.ID)
			return res, ghcall.Refuse(authError(a), now)
		default:
			res.Still = append(res.Still, m.Org)
			if first == nil && !ghcall.Is(err, ghcall.AppNotInstalled) {
				first = err
			}
		}
	}
	if first != nil {
		return res, ghcall.Refuse(first, now)
	}
	return res, nil
}
