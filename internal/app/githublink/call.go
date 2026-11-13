package githublink

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// call は、Store のアカウントで repo について GitHub に問い合わせる 1 か所(D4b-12)。トークンが拒否されたら
// 認証エラーにする(D4a の lookup・readMany と同じ)。AppNotInstalled なら記録を置き、成功なら repo の owner の
// 記録を消す。記録が変わったら EventGitHub(Store は空)を流す。
func (l Links) call(ctx context.Context, store port.StoreID, repo string, f func(token string) error) error {
	tok, a, err := l.Tokens.TokenFor(ctx, store, repo)
	if err != nil {
		return err
	}
	err = f(tok)
	if ghcall.Is(err, ghcall.TokenRejected) {
		l.Tokens.Rejected(a.ID)
		return authError(a)
	}
	var fail *ghcall.Failure
	var changed bool
	var recErr error
	switch {
	case err == nil:
		owner, _, _ := strings.Cut(repo, "/")
		changed, recErr = l.Missing.Clear(ctx, a.ID, owner)
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
