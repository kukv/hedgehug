package githublink

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// ScanRepo は、Store の repo を、間隔と読まない期限によらず 1 回探す(D4b-9)。探さない Store(D4b-10)では何もしない。
// 取り直しの記録は変えない。自動でリンクしたか、記録の失敗の文言が変わったら EventGitHub を流す。
func (l Links) ScanRepo(ctx context.Context, store port.StoreID, repo string) error {
	s, ok, err := l.storeInfo(ctx, store)
	if err != nil || !ok || s.ReadOnly || s.Hidden || s.Account == "" {
		return err
	}
	scans, err := l.Scans.Scans(ctx, store)
	if err != nil {
		return err
	}
	rd := &reading{l: l, now: l.now(), limited: map[account.ID]limit{}}
	r := &repoLinks{key: strings.ToLower(repo), repo: repo, scan: true}
	res := repoResult{repo: r}
	res.pulls, res.scanText, res.cause = rd.scan(ctx, s, r, scans[r.key])
	changed, err := l.writeScans(ctx, store, []repoResult{res}, scans, newPass())
	if changed {
		l.Events.Publish(port.Event{Kind: port.EventGitHub, Store: store})
	}
	return errors.Join(res.cause, err)
}

// RepoFailures は、プロジェクトのリポジトリごとの「PR を探せませんでした: 」に続ける文言(D4b-10)。キーはリポジトリの
// 書き方のまま。失敗のないリポジトリは入れない。プロジェクトがないか削除済みなら port.ErrNotFound。
func (l Links) RepoFailures(ctx context.Context, store port.StoreID, id project.ID) (map[string]string, error) {
	var p project.Project
	err := l.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		p, err = port.LiveProject(r, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	s, _, err := l.storeInfo(ctx, store)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	switch {
	case s.ReadOnly:
		return out, nil
	case s.Account == "" && !s.Hidden:
		// 読めるアカウントがない: 記録を置かずに、その場で作る
		for _, repo := range p.Repos {
			out[repo] = l.failMessage(&ghcall.Failure{Kind: ghcall.NoAccount, Repo: repo})
		}
		return out, nil
	}
	scans, err := l.Scans.Scans(ctx, store)
	if err != nil {
		return nil, err
	}
	for _, repo := range p.Repos {
		if text := scans[strings.ToLower(repo)].Failure; text != "" {
			out[repo] = text
		}
	}
	return out, nil
}

// storeInfo は、store の GitHub の設定。一覧になければ ok は false。
func (l Links) storeInfo(ctx context.Context, store port.StoreID) (port.StoreGitHubInfo, bool, error) {
	infos, err := l.StoreGH.List(ctx)
	if err != nil {
		return port.StoreGitHubInfo{}, false, err
	}
	for _, s := range infos {
		if s.ID == store {
			return s, true, nil
		}
	}
	return port.StoreGitHubInfo{}, false, nil
}
