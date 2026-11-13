package githublink

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// Refresh は、詳細パネルを開いたときに、アイテムのリンクを GitHub から取り直す(07「表示と更新」。D3-8)。
// 値が変わったリンクは、リンクごとに実行者「GitHub」で保存する(D3-10)。失敗した行には理由を添える。
// 閲覧のみの Store と、このマシンにアカウントが 1 つもないときは問い合わせず、失敗の文言も出さない(D3-9)。
func (l Links) Refresh(ctx context.Context, store port.StoreID, id item.ID) ([]LinkRow, error) {
	skip, err := l.skipRefresh(ctx, store)
	if err != nil {
		return nil, err
	}
	if skip {
		// 取り直しをしないので、置いてある失敗の文言は出さない(消しもしない)。
		rows, err := l.List(ctx, store, id)
		for i := range rows {
			rows[i].Error = ""
		}
		return rows, err
	}
	v, err := l.view(ctx, store, id)
	if err != nil {
		return nil, err
	}
	changed, err := l.refreshAll(ctx, store, id, v)
	if err != nil {
		return nil, err
	}
	if changed {
		l.Events.Publish(port.Event{Kind: port.EventGitHub, Store: store})
	}
	return l.List(ctx, store, id)
}

// refreshAll は、まずリンクをすべて GitHub から読み、そのあとリンクごとに保存する(D4a-14)。失敗した行には文言を
// 置き、取り直せた行の文言は消す(D4a-11)。チェックの結果か失敗の文言が変わったら true。
func (l Links) refreshAll(ctx context.Context, store port.StoreID, id item.ID, v itemView) (bool, error) {
	p := newPass()
	found := l.lookupAll(ctx, store, id, v.links.Links, p)
	changed := false
	for i, link := range v.links.Links {
		if found[i].text == "" {
			c, err := l.writeFound(ctx, store, id, link, found[i].found, p)
			if err != nil {
				return false, err
			}
			changed = changed || c
		}
		c, err := l.setFailure(ctx, store, link.Key(), found[i].text)
		if err != nil {
			return false, err
		}
		changed = changed || c
	}
	return changed, nil
}

// lookupResult は、1 本のリンクを読んだ結果。text が空でなければ失敗の文言。
type lookupResult struct {
	found port.Found
	text  string
}

// lookupAll は、アイテム id の links を 1 本ずつ読み、読めた値を p に覚える。利用上限に当たったら、残りのリンクは
// 問い合わせず、同じ文言にする。
func (l Links) lookupAll(ctx context.Context, store port.StoreID, id item.ID, links []itemlink.Link, p *pass) []lookupResult {
	out := make([]lookupResult, len(links))
	limited := ""
	for i, link := range links {
		if limited != "" {
			out[i].text = limited
			continue
		}
		f, err := l.lookup(ctx, store, link.Repo, link.Number)
		if err != nil {
			out[i].text = l.failText(err)
			if ghcall.Is(err, ghcall.RateLimited) {
				limited = out[i].text
			}
			continue
		}
		out[i].found = f
		p.put(id, link, f.Values)
	}
	return out
}

// setFailure は、リンク key の取り直しの失敗の文言を置く。text が空なら消す。前と変わったら true(D4a-11)。
func (l Links) setFailure(ctx context.Context, store port.StoreID, key, text string) (bool, error) {
	return l.Reads.SetFailure(ctx, store, key, text)
}

// skipRefresh は、問い合わせないときか(D3-9)。
func (l Links) skipRefresh(ctx context.Context, store port.StoreID) (bool, error) {
	infos, err := l.StoreGH.List(ctx)
	if err != nil {
		return false, err
	}
	for _, s := range infos {
		if s.ID == store && s.ReadOnly {
			return true, nil
		}
	}
	accounts, err := l.Accounts.List(ctx)
	return len(accounts) == 0, err
}
