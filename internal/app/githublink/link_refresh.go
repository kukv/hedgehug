package githublink

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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

// refreshAll は、リンクを 1 本ずつ取り直し、失敗した行には文言を置き、取り直せた行の文言は消す(D4a-11)。
// チェックの結果か失敗の文言が変わったら true。利用上限に当たったら、残りのリンクは問い合わせず、同じ文言を置く。
func (l Links) refreshAll(ctx context.Context, store port.StoreID, id item.ID, v itemView) (bool, error) {
	changed, limited := false, ""
	for _, link := range v.links.Links {
		text := limited
		if text == "" {
			c, err := l.refreshLink(ctx, store, id, v, link)
			if err != nil {
				return false, err
			}
			changed = changed || c.changed
			text = c.text
			if c.limited {
				limited = text
			}
		}
		c, err := l.setFailure(ctx, store, link.Key(), text)
		if err != nil {
			return false, err
		}
		changed = changed || c
	}
	return changed, nil
}

// linkResult は、1 本のリンクの取り直しの結果。text が空でなければ失敗(limited は利用上限)。
type linkResult struct {
	changed bool // チェックの結果が変わった
	text    string
	limited bool
}

func (l Links) refreshLink(ctx context.Context, store port.StoreID, id item.ID, v itemView, link itemlink.Link) (linkResult, error) {
	f, err := l.lookup(ctx, store, link.Repo, link.Number)
	if err != nil {
		return linkResult{text: l.failText(err), limited: ghcall.Is(err, ghcall.RateLimited)}, nil
	}
	c, err := l.refreshOne(ctx, store, id, v, link, f)
	return linkResult{changed: c}, err
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

// errRenameTaken は、保存の中で、改名先が同じアイテムの別のリンクと重なったこと(改名せずに値だけ取り直す)。
var errRenameTaken = errors.New("github: rename target is already linked")

// refreshOne は、1 本のリンクを f の値にそろえて保存し、そのあとの名前でチェックの結果を置く。古い名前の
// チェックの結果は消さない(同じ Store の別のアイテムが同じ PR をリンクしていることがある)。チェックの結果が
// 変わったら true。取り直すあいだに外されたリンクは何もしない。
func (l Links) refreshOne(ctx context.Context, store port.StoreID, id item.ID, v itemView, link itemlink.Link, f port.Found) (bool, error) {
	repo, err := l.renameTo(ctx, store, v, link, f.Repo)
	if err != nil {
		return false, err
	}
	kept, err := l.writeRefresh(ctx, store, id, v.display, link, repo, f.Values)
	if errors.Is(err, errRenameTaken) {
		repo = link.Repo
		kept, err = l.writeRefresh(ctx, store, id, v.display, link, repo, f.Values)
	}
	if err != nil || !kept {
		return false, err
	}
	return l.saveChecks(ctx, store, itemlink.Ref{Kind: link.Kind, Repo: repo, Number: link.Number}, f.Checks)
}

// writeRefresh は、link を値 v で(repo が違えば改名して)実行者「GitHub」で保存する。値が同じなら保存しない。
// 取り直すあいだに外された・書き換えられたリンクなら何もせず false。改名先が、保存の中で同じアイテムの
// 別のリンクと重なれば errRenameTaken(何も保存しない)。
func (l Links) writeRefresh(ctx context.Context, store port.StoreID, id item.ID, display string, link itemlink.Link, repo string, v itemlink.Values) (bool, error) {
	summary := itemlink.RefreshSummary(display, link, repo, v)
	if summary == "" {
		return true, nil
	}
	kept := false
	op := port.Operation{Actor: change.Actor{Kind: change.ActorGitHub}, Summary: summary}
	err := l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		kept = false
		cur, err := readItem(tx, id)
		if err != nil {
			return err
		}
		now, ok := cur.links.Find(link.Ref)
		if !ok || now.Repo != link.Repo || now.Kind != link.Kind {
			return nil
		}
		kept = true
		if repo == now.Repo {
			return tx.WriteItems().Append(id, cur.links.Refresh(now, v)...)
		}
		if _, taken := cur.links.Find(itemlink.Ref{Kind: now.Kind, Repo: repo, Number: now.Number}); taken {
			return errRenameTaken
		}
		return tx.WriteItems().Append(id, cur.links.Rename(now, repo, v)...)
	})
	return kept, err
}

// renameTo は、書き換えるリポジトリ名。改名・移管がない(大文字小文字だけの違いを含む)か、書き換えると
// 同じアイテムの別のリンク・別のアイテムの Issue と重なるときは、いまの名前(D3-11)。
func (l Links) renameTo(ctx context.Context, store port.StoreID, v itemView, link itemlink.Link, repo string) (string, error) {
	if strings.EqualFold(repo, link.Repo) {
		return link.Repo, nil
	}
	moved := itemlink.Ref{Kind: link.Kind, Repo: repo, Number: link.Number}
	if _, ok := v.links.Find(moved); ok {
		return link.Repo, nil
	}
	if link.Kind == itemlink.Issue {
		_, taken, err := l.issueOwner(ctx, store, moved, v.item.ID)
		if err != nil || taken {
			return link.Repo, err
		}
	}
	return repo, nil
}
