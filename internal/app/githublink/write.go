package githublink

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// errRenameTaken は、保存の中で、改名先が同じアイテムの別のリンクと重なったこと(改名せずに値だけ取り直す)。
var errRenameTaken = errors.New("github: rename target is already linked")

// writeFound は、取り直した値 f をリンク link に書く 1 回の保存(実行者「GitHub」)と、その中のきっかけ。
// D3 の refreshOne・writeRefresh・renameTo をここへまとめ、間隔ごとの取り直し(Task 12)も使う。
// 値が変わらなければ保存もきっかけもない。チェックの結果はいまと同じに置く。
// チェックの結果は保存のあとの名前で置き、古い名前のチェックの結果は消さない(同じ Store の別のアイテムが
// 同じ PR をリンクしていることがある)。取り直すあいだに外されたリンクは何もしない。動いたルールの通知は、
// 保存のあとにここで、チェックの結果を置く前に書く(置くのに失敗しても通知は失わない)。
func (l Links) writeFound(ctx context.Context, store port.StoreID, id item.ID, link itemlink.Link, f port.Found, p *pass) (checksChanged bool, err error) {
	v, err := l.view(ctx, store, id)
	if err != nil {
		return false, err
	}
	repo, err := l.renameTo(ctx, store, v, link, f.Repo)
	if err != nil {
		return false, err
	}
	w := foundWrite{id: id, display: v.display, link: link, repo: repo, values: f.Values, p: p}
	kept, fs, err := l.writeRefresh(ctx, store, w)
	if errors.Is(err, errRenameTaken) {
		w.repo = link.Repo
		kept, fs, err = l.writeRefresh(ctx, store, w)
	}
	if err != nil || !kept {
		return false, err
	}
	nerr := l.notify(ctx, store, fs)
	checksChanged, cerr := l.saveChecks(ctx, store, itemlink.Ref{Kind: link.Kind, Repo: w.repo, Number: link.Number}, f.Checks)
	return checksChanged, errors.Join(nerr, cerr)
}

// foundWrite は、1 本のリンクの取り直しの保存の中身。
type foundWrite struct {
	id      item.ID
	display string
	link    itemlink.Link
	repo    string
	values  itemlink.Values
	p       *pass
}

// writeRefresh は、link を値 values で(repo が違えば改名して)実行者「GitHub」で保存し、同じ保存の中で
// きっかけを動かす。値が同じなら保存しない。取り直すあいだに外された・書き換えられたリンクなら何もせず false。
// 読んだあとで同期や別の取り直しがリンクの値を変えていたら(いまの値が読む前の w.link.Values と違う)、読んだ値は
// それより古いことがあるので、書かずにきっかけも動かさず false(次の取り直しで合わせる)。
// 改名先が、保存の中で同じアイテムの別のリンクと重なれば errRenameTaken(何も保存しない)。
func (l Links) writeRefresh(ctx context.Context, store port.StoreID, w foundWrite) (bool, []fired, error) {
	summary := itemlink.RefreshSummary(w.display, w.link, w.repo, w.values)
	if summary == "" {
		return true, nil, nil
	}
	kept := false
	var fs []fired
	op := port.Operation{Actor: change.Actor{Kind: change.ActorGitHub}, Summary: summary}
	err := l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		kept, fs = false, nil
		cur, err := readItem(tx, w.id)
		if err != nil {
			return err
		}
		now, ok := cur.links.Find(w.link.Ref)
		if !ok || now.Repo != w.link.Repo || now.Kind != w.link.Kind || now.Values != w.link.Values {
			return nil
		}
		kept = true
		ds := cur.links.Refresh(now, w.values)
		if w.repo != now.Repo {
			if _, taken := cur.links.Find(itemlink.Ref{Kind: now.Kind, Repo: w.repo, Number: now.Number}); taken {
				return errRenameTaken
			}
			ds = cur.links.Rename(now, w.repo, w.values)
		}
		if err := tx.WriteItems().Append(w.id, ds...); err != nil {
			return err
		}
		after := itemlink.Link{Ref: itemlink.Ref{Kind: now.Kind, Repo: w.repo, Number: now.Number}, Values: w.values}
		fs, err = fire(tx, w.id, &now, after, w.p)
		return err
	})
	if err != nil {
		return false, nil, err
	}
	return kept, fs, nil
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
