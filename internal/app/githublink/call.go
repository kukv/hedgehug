package githublink

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// call は、Store のアカウントで repo について GitHub に問い合わせる 1 か所(D4b-12)。f は答えのリポジトリ名
// (改名・移管のあとの名前。分からなければ空)を返す。トークンが拒否されたら認証エラーにする(D4a の lookup・
// readMany と同じ)。AppNotInstalled なら記録を置き、成功なら、問い合わせた名前か答えの名前を記録した記録だけを
// 消す(同じ組織のほかのリポジトリが読めても消さない。改名のあとの記録は答えの名前で置かれる)。記録が変わったら
// EventGitHub(Store は空)を流す。成功で答えの名前が問い合わせた名前と違えば、Store の改名の対応を記録する
// (自動リンクの重なりの判定。D4b 最終レビュー R-1)。
func (l Links) call(ctx context.Context, store port.StoreID, repo string, f func(token string) (string, error)) error {
	tok, a, err := l.Tokens.TokenFor(ctx, store, repo)
	if err != nil {
		return err
	}
	answer, err := f(tok)
	if ghcall.Is(err, ghcall.TokenRejected) {
		l.Tokens.Rejected(a.ID)
		return authError(a)
	}
	var fail *ghcall.Failure
	var changed bool
	var recErr error
	switch {
	case err == nil:
		changed, recErr = l.clearMissing(ctx, a.ID, repo, answer)
		if answer != "" && !strings.EqualFold(answer, repo) {
			recErr = errors.Join(recErr, l.Scans.SaveRename(ctx, store, repo, answer))
		}
	case ghcall.Is(err, ghcall.AppNotInstalled) && errors.As(err, &fail):
		changed, recErr = l.Missing.Put(ctx, a.ID, port.MissingOrg{Org: fail.Org, Repo: fail.Repo})
	}
	if changed {
		l.Events.Publish(port.Event{Kind: port.EventGitHub})
	}
	if recErr != nil {
		return errors.Join(err, recErr)
	}
	return err
}

// clearMissing は、読めたリポジトリ(問い合わせた名前 repo と、違えば答えの名前 answer)の記録を消す。
func (l Links) clearMissing(ctx context.Context, a account.ID, repo, answer string) (bool, error) {
	names := []string{repo}
	if answer != "" && !strings.EqualFold(answer, repo) {
		names = append(names, answer)
	}
	changed := false
	for _, r := range names {
		owner, _, _ := strings.Cut(r, "/")
		c, err := l.Missing.Clear(ctx, a, owner, r)
		if err != nil {
			return changed, err
		}
		changed = changed || c
	}
	return changed, nil
}
