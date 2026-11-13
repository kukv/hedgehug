package githublink

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/ghread"
)

// ReadOnce は、自動で読む Store(D4a-9)の、リンクのあるリポジトリのうち、読む時期が来たもの(force なら全部)を 1 回読む。
// every は「GitHub を読む間隔」。1 つの Store・リポジトリの失敗で、ほかを止めない。
// Store ごとに、まず読む時期が来たリポジトリをすべて読み、そのあとでリンクを書く(D4a-14)。保存に失敗した Store は、
// その pass を捨てて残りを書かない(pass は保存の前にきっかけを覚えるため)。エラーは最後にまとめて返す。
func (l Links) ReadOnce(ctx context.Context, every time.Duration, force bool) error {
	infos, err := l.StoreGH.List(ctx)
	if err != nil {
		return err
	}
	rd := &reading{l: l, every: every, force: force, now: l.now(), limited: map[account.ID]limit{}}
	var errs []error
	for _, s := range infos {
		if s.ReadOnly || s.Hidden || s.Account == "" {
			continue // 自動で読まない Store(D4a-9)。文言も置かない
		}
		changed, err := rd.readStore(ctx, s)
		if changed {
			l.Events.Publish(port.Event{Kind: port.EventGitHub, Store: s.ID})
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("github read %s: %w", s.ID, err))
		}
	}
	return errors.Join(errs...)
}

// reading は、1 回の ReadOnce。limited は、利用上限で断られたアカウントと、その文言・期限(D4a-6)。
// 上限はトークンごとなので、Store をまたいで覚える。
type reading struct {
	l       Links
	every   time.Duration
	force   bool
	now     time.Time
	limited map[account.ID]limit
}

// limit は、利用上限で断られたときの文言(07 の表の文。前置きなし)と、読まない期限。
type limit struct {
	text      string
	notBefore time.Time
}

// repoResult は、1 つのリポジトリを読んだ結果。text が空でなければ、取り直しのリポジトリ全体の失敗の文言。
// pulls は探して見つけた PR、scanText は探せなかったときの文言(07 の表の文。D4b-4)。
// cause は、ghcall の失敗でないエラー(行の文言は「接続できません」にし、ReadOnce のエラーにも入れる)。
type repoResult struct {
	repo     *repoLinks
	text     string
	batch    port.Batch
	pulls    port.Pulls
	scanText string
	cause    error
}

// readStore は、Store s の読む時期が来たリポジトリを読み(取り直しと PR の探索)、記録を置いてから、リンクを書き、
// そのあとで PR を自動でリンクする(D4b-8)。チェックの結果か失敗の文言が変わったか、自動でリンクしたら true。
// ghcall の失敗でないエラーは、書いたあとでまとめて返す(Reader がログに出す)。
func (rd *reading) readStore(ctx context.Context, s port.StoreGitHubInfo) (bool, error) {
	repos, err := rd.l.collect(ctx, s.ID)
	if errors.Is(err, port.ErrStoreNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	records, err := rd.l.Reads.Records(ctx, s.ID)
	if err != nil {
		return false, err
	}
	scans, err := rd.l.Scans.Scans(ctx, s.ID)
	if err != nil {
		return false, err
	}
	p := newPass()
	var results []repoResult
	var causes []error
	for _, r := range repos {
		rec := records[r.key]
		if !rd.force && !ghread.Due(rd.now, rec, rd.every) {
			continue
		}
		res, next := rd.fetch(ctx, s, r, rec, p)
		rec = rd.scanInto(ctx, s, &res, scans[r.key], rec, next)
		if err := rd.l.Reads.SaveRecord(ctx, s.ID, r.key, rec); err != nil {
			return false, err
		}
		results = append(results, res)
		causes = append(causes, res.cause)
	}
	changed, err := rd.l.writeResults(ctx, s.ID, results, p)
	if err == nil {
		var c bool
		c, err = rd.l.writeScans(ctx, s.ID, results, scans, p)
		changed = changed || c
	}
	return changed, errors.Join(append(causes, err)...)
}

// fetch は、リポジトリ r を読み、結果と、置く記録を返す(D4a-5・6)。読めた値は p.fetched に入れる。
// 利用上限で断られたら、試した時刻は進めずに読まない期限を置く(期限が間隔より早くても、期限に読めるように)。
// 同じアカウントがこの読み取りで断られていれば、問い合わせずに同じ文言と期限にする。
// 取り直す番号がなければ(探すだけのリポジトリ)問い合わせず、前の記録に試した時刻だけを置く。
func (rd *reading) fetch(ctx context.Context, s port.StoreGitHubInfo, r *repoLinks, rec ghread.Record, p *pass) (repoResult, ghread.Record) {
	if lim, ok := rd.limited[s.Account]; ok {
		rec.Failed, rec.NotBefore = true, lim.notBefore
		return repoResult{repo: r, text: refreshText(lim.text)}, rec
	}
	if len(r.numbers) == 0 {
		rec.Attempt, rec.Failed, rec.NotBefore = rd.now, false, time.Time{}
		return repoResult{repo: r}, rec
	}
	b, err := rd.l.readMany(ctx, s.ID, r)
	var f *ghcall.Failure
	if ghcall.Is(err, ghcall.RateLimited) && errors.As(err, &f) {
		lim := rd.limit(s.Account, err, f)
		rec.Failed, rec.NotBefore = true, lim.notBefore
		return repoResult{repo: r, text: refreshText(lim.text)}, rec
	}
	if err != nil {
		rec.Attempt, rec.Failed, rec.NotBefore = rd.now, true, time.Time{}
		res := repoResult{repo: r, text: rd.l.failText(err)}
		if !errors.As(err, &f) {
			res.cause = err
		}
		return res, rec
	}
	for _, t := range r.targets {
		if found, ok := b.Found[t.link.Number]; ok {
			p.put(t.item, t.link, found.Values)
		}
	}
	rec = ghread.Record{Attempt: rd.now, Limit: b.Rate.Limit, Remaining: b.Rate.Remaining, ResetAt: b.Rate.ResetAt}
	return repoResult{repo: r, batch: b}, rec
}

// readMany は、Store のアカウントで r の番号をまとめて読む。トークンが拒否されたら認証エラーにする(07「状態」)。
func (l Links) readMany(ctx context.Context, store port.StoreID, r *repoLinks) (port.Batch, error) {
	var b port.Batch
	err := l.call(ctx, store, r.repo, func(tok string) (string, error) {
		var err error
		b, err = l.GitHub.ReadMany(ctx, tok, r.repo, r.numbers)
		return b.Repo, err
	})
	if err != nil {
		return port.Batch{}, err
	}
	return b, nil
}

// writeResults は、読んだ結果をリンクごとに書く(D4a-12)。保存に失敗したら、残りは書かずに返す。
func (l Links) writeResults(ctx context.Context, store port.StoreID, results []repoResult, p *pass) (bool, error) {
	changed := false
	for _, res := range results {
		for _, t := range res.repo.targets {
			c, err := l.writeTarget(ctx, store, t, res, p)
			changed = changed || c
			if err != nil {
				return changed, err
			}
		}
	}
	return changed, nil
}

// writeTarget は、1 本のリンクに、読んだ値を書いて文言を消すか、失敗の文言を置く。答えになかった番号は
// 「見つかりません」の文言にする。読んだあとで削除されたアイテム(プロジェクト)のリンクは、何も書かずに飛ばす。
// writeFound が返す port.ErrNotFound は、アイテムかそのプロジェクト(ワークスペース)が消えたことだけを表す。
// writeFound が使うストア、受信箱、チェックは、それ以外では返さない。
func (l Links) writeTarget(ctx context.Context, store port.StoreID, t target, res repoResult, p *pass) (bool, error) {
	text, changed := res.text, false
	if text == "" {
		found, ok := res.batch.Found[t.link.Number]
		if !ok {
			text = l.failText(&ghcall.Failure{Kind: ghcall.NotFound, Repo: t.link.Repo, Number: t.link.Number})
		} else {
			c, err := l.writeFound(ctx, store, t.item, t.link, found, p)
			if errors.Is(err, port.ErrNotFound) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			changed = c
		}
	}
	c, err := l.setFailure(ctx, store, t.link.Key(), text)
	return changed || c, err
}
