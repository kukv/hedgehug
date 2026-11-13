package githublink

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/ghread"
)

// limit は、アカウント a が利用上限で断られたこと(失敗 f)を覚え、その文言と読まない期限を返す(D4a-6)。
func (rd *reading) limit(a account.ID, err error, f *ghcall.Failure) limit {
	nb := ghread.Limited(rd.now, f.ResetAt)
	if !nb.After(rd.now) {
		// このマシンの時計が進んでいて戻る時刻が過ぎているときも、すぐには読み直さない(分からないときと同じ 1 分後)
		nb = rd.now.Add(time.Minute)
	}
	lim := limit{text: rd.l.failMessage(err), notBefore: nb}
	rd.limited[a] = lim
	return lim
}

// scanInto は、取り直しのあとで res のリポジトリの PR を探し(D4b-8)、結果を res に入れ、読み取りの記録 rec を直す。
// 利用上限で断られたら(取り直しで断られたときを含む)、試した時刻は進めずに読まない期限を置く。成功なら
// 利用上限は探索の答えのものにする。取り直す番号のないリポジトリで失敗したら、失敗の印を置く。
func (rd *reading) scanInto(ctx context.Context, s port.StoreGitHubInfo, res *repoResult, srec port.ScanRecord, rec ghread.Record) ghread.Record {
	pulls, text, cause := rd.scan(ctx, s, res.repo, srec)
	res.pulls, res.scanText, res.cause = pulls, text, errors.Join(res.cause, cause)
	switch lim, limited := rd.limited[s.Account]; {
	case limited:
		rec.Failed, rec.NotBefore = true, lim.notBefore
	case text == "":
		rec.Limit, rec.Remaining, rec.ResetAt = pulls.Rate.Limit, pulls.Rate.Remaining, pulls.Rate.ResetAt
	case len(res.repo.numbers) == 0:
		rec.Failed = true
	}
	return rec
}

// scan は、Store s のアカウントでリポジトリ r の PR を読む(D4b-4)。初めてなら新しい順に 100 件、2 回目からは
// 記録の時刻以上に更新されたものをすべて。失敗なら 07 の表の文を返す(ghcall の失敗でなければ「接続できません」の
// 文と、そのエラー)。同じアカウントがこの読み取りで利用上限に断られていれば、問い合わせずに同じ文。
func (rd *reading) scan(ctx context.Context, s port.StoreGitHubInfo, r *repoLinks, rec port.ScanRecord) (port.Pulls, string, error) {
	if lim, ok := rd.limited[s.Account]; ok {
		return port.Pulls{}, lim.text, nil
	}
	since := rec.Since
	if rec.Read && since.IsZero() {
		// 探し終えたが PR が 1 本もなかった: 前回のあとの PR をすべて見る(ゼロは「100 件だけ」になる)
		since = time.Unix(0, 0).UTC()
	}
	pulls, err := rd.l.readPulls(ctx, s.ID, r.repo, since)
	var f *ghcall.Failure
	switch {
	case err == nil:
		return pulls, "", nil
	case ghcall.Is(err, ghcall.RateLimited) && errors.As(err, &f):
		return port.Pulls{}, rd.limit(s.Account, err, f).text, nil
	case errors.As(err, &f):
		return port.Pulls{}, rd.l.failMessage(err), nil
	default:
		return port.Pulls{}, rd.l.failMessage(err), err
	}
}

// readPulls は、Store のアカウントで repo の PR を読む(D4b-12 の call を通す)。
func (l Links) readPulls(ctx context.Context, store port.StoreID, repo string, since time.Time) (port.Pulls, error) {
	var ps port.Pulls
	err := l.call(ctx, store, repo, func(tok string) error {
		var err error
		ps, err = l.GitHub.ReadPulls(ctx, tok, repo, since)
		return err
	})
	return ps, err
}

// writeScans は、取り直しを書いたあとで、探した結果ごとに PR を自動でリンクし、探した記録を置く(D4b-4・7・8)。
// 自動でリンクしたか、記録の失敗の文言が変わったら true。保存に失敗したら、そのリポジトリの記録を進めずに
// 残りを書かずに返す(次の読み取りで同じ PR を見直す)。
func (l Links) writeScans(ctx context.Context, store port.StoreID, results []repoResult, scans map[string]port.ScanRecord, p *pass) (bool, error) {
	var items storeItems
	if slices.ContainsFunc(results, func(r repoResult) bool { return r.repo.scan && len(r.pulls.Pulls) > 0 }) {
		var err error
		if items, err = l.readStoreItems(ctx, store); err != nil {
			return false, err
		}
	}
	changed := false
	for _, res := range results {
		if !res.repo.scan {
			continue
		}
		c, err := l.writeScan(ctx, store, res, scans[res.repo.key], items, p)
		changed = changed || c
		if err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// writeScan は、1 つのリポジトリの探した結果 res を書く。old は前の記録。成功なら PR を自動でリンクしてから、
// 読んだ印とどこまで見たかを進め、失敗なら文言だけを置く(D4b-4)。
func (l Links) writeScan(ctx context.Context, store port.StoreID, res repoResult, old port.ScanRecord, items storeItems, p *pass) (bool, error) {
	next := old
	next.Failure = res.scanText
	linked := false
	if res.scanText == "" {
		var err error
		if linked, err = l.linkPulls(ctx, store, res.pulls, items, !old.Read, p); err != nil {
			return linked, err
		}
		next.Read, next.Since = true, latest(old.Since, res.pulls.Pulls)
	}
	if _, err := l.Scans.SaveScan(ctx, store, res.repo.key, next); err != nil {
		return linked, err
	}
	return linked || next.Failure != old.Failure, nil
}

// readStoreItems は、Store の削除済みでないアイテムの表を読む。
func (l Links) readStoreItems(ctx context.Context, store port.StoreID) (storeItems, error) {
	var items storeItems
	err := l.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		items, err = readStoreItems(r)
		return err
	})
	return items, err
}

// linkPulls は、見つけた PR をそれぞれ自動でリンクする。first はそのリポジトリを初めて探したか(D4b-7)。
func (l Links) linkPulls(ctx context.Context, store port.StoreID, ps port.Pulls, items storeItems, first bool, p *pass) (bool, error) {
	changed := false
	for _, pull := range ps.Pulls {
		c, err := l.autoLink(ctx, store, ps.Repo, pull, items, first, p)
		changed = changed || c
		if err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// latest は、since と、pulls の更新の時刻のうち、いちばん後のもの。
func latest(since time.Time, pulls []port.Pull) time.Time {
	for _, p := range pulls {
		if p.UpdatedAt.After(since) {
			since = p.UpdatedAt
		}
	}
	return since
}
