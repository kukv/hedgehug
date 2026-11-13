package github

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// Refresh は、詳細パネルを開いたときに、アイテムのリンクを GitHub から取り直す(07「表示と更新」。D3-8)。
// 値が変わったリンクは、リンクごとに実行者「GitHub」で保存する(D3-10)。失敗した行には理由を添える。
// 閲覧のみの Store と、このマシンにアカウントが 1 つもないときは問い合わせない(D3-9)。
func (l Links) Refresh(ctx context.Context, store port.StoreID, id item.ID) ([]LinkRow, error) {
	skip, err := l.skipRefresh(ctx, store)
	if err != nil {
		return nil, err
	}
	if skip {
		return l.List(ctx, store, id)
	}
	v, err := l.view(ctx, store, id)
	if err != nil {
		return nil, err
	}
	errs := map[string]string{}
	checksChanged := false
	for _, link := range v.links.Links {
		f, err := l.lookup(ctx, store, link.Repo, link.Number)
		if err != nil {
			errs[link.Key()] = l.failText(err)
			continue
		}
		ref, err := l.refreshOne(ctx, store, id, v, link, f)
		if err != nil {
			return nil, err
		}
		changed, err := l.saveChecks(ctx, store, ref, f.Checks)
		if err != nil {
			return nil, err
		}
		checksChanged = checksChanged || changed
	}
	if checksChanged {
		l.Events.Publish(port.Event{Kind: port.EventGitHub, Store: store})
	}
	rows, err := l.List(ctx, store, id)
	for i := range rows {
		rows[i].Error = errs[rows[i].Key()]
	}
	return rows, err
}

// skipRefresh は、問い合わせないときか(D3-9)。
func (l Links) skipRefresh(ctx context.Context, store port.StoreID) (bool, error) {
	infos, err := l.Connector.Stores.List(ctx)
	if err != nil {
		return false, err
	}
	for _, s := range infos {
		if s.ID == store && s.ReadOnly {
			return true, nil
		}
	}
	accounts, err := l.Connector.Accounts.List(ctx)
	return len(accounts) == 0, err
}

// refreshOne は、1 本のリンクを f の値にそろえて保存し、そのあとのリンクの Ref を返す。値が同じなら保存しない。
func (l Links) refreshOne(ctx context.Context, store port.StoreID, id item.ID, v itemView, link itemlink.Link, f port.Found) (itemlink.Ref, error) {
	repo, err := l.renameTo(ctx, store, v, link, f.Repo)
	if err != nil {
		return itemlink.Ref{}, err
	}
	summary := itemlink.RefreshSummary(v.display, link, repo, f.Values)
	after := itemlink.Ref{Kind: link.Kind, Repo: repo, Number: link.Number}
	if summary == "" {
		return after, nil
	}
	op := port.Operation{Actor: change.Actor{Kind: change.ActorGitHub}, Summary: summary}
	err = l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := readItem(tx, id)
		if err != nil {
			return err
		}
		now, ok := cur.links.Find(link.Ref)
		if !ok || now.Repo != link.Repo || now.Kind != link.Kind {
			return nil // 取り直すあいだに外された・書き換えられた
		}
		drafts := cur.links.Refresh(now, f.Values)
		if repo != now.Repo {
			drafts = cur.links.Rename(now, repo, f.Values)
		}
		return tx.WriteItems().Append(id, drafts...)
	})
	return after, err
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
