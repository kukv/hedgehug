package githublink

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// autoLink は、PR pull を、Store の items が指すアイテムに自動でリンクする(D4b-5〜7)。Closed の PR は何もしない。
// asked は問い合わせた名前、repo は GitHub の nameWithOwner(改名・移管のあとは違う。リンクはこの名前で書く)。
// アイテムごとに 1 回の保存(実行者「GitHub」)。first なら(そのリポジトリを初めて探した)きっかけを起こさない。
// p が nil でなければ、同じ読み取りのきっかけの重なりを除く。どれかのアイテムにリンクを書いたら true。
func (l Links) autoLink(ctx context.Context, store port.StoreID, asked, repo string, pull port.Pull, items storeItems, first bool, p *pass) (bool, error) {
	if pull.State == itemlink.Closed {
		return false, nil
	}
	link := itemlink.Link{Ref: itemlink.Ref{Kind: itemlink.PR, Repo: repo, Number: pull.Number}, Values: pull.Values}
	old := itemlink.Ref{Kind: itemlink.PR, Repo: asked, Number: pull.Number}
	linked := false
	var errs []error // 保存のあとの通知の失敗(ほかのアイテムの保存は続ける)
	for _, id := range items.targets(pull) {
		ok, fs, err := l.saveAutoLink(ctx, store, id, items.live[id], link, old, first, p)
		if ok {
			linked = true
			errs = append(errs, l.notify(ctx, store, fs))
		}
		if err != nil {
			errs = append(errs, err)
			break
		}
	}
	if linked {
		// チェックの結果は PR ごとに 1 つ(同じ Store のアイテムで共有する)。途中で保存に失敗しても、書いたリンクの分は置く
		_, err := l.saveChecks(ctx, store, link.Ref, pull.Checks)
		errs = append(errs, err)
	}
	return linked, errors.Join(errs...)
}

// saveAutoLink は、アイテム id(画面の ID は display)に link を書く 1 回の保存と、その中のきっかけ。
// 保存の中で読み直し、削除済み・同じ番号のリンクがある・解除の記録にあるときは書かず false。old は問い合わせた
// 名前の同じ PR(link と old のどちらかに当たれば書かない)。
func (l Links) saveAutoLink(ctx context.Context, store port.StoreID, id item.ID, display string, link itemlink.Link, old itemlink.Ref, first bool, p *pass) (bool, []fired, error) {
	wrote := false
	var fs []fired
	op := port.Operation{Actor: change.Actor{Kind: change.ActorGitHub}, Summary: itemlink.AutoSummary(display, link.Ref)}
	err := l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		wrote, fs = false, nil
		cur, err := readItem(tx, id)
		if errors.Is(err, port.ErrNotFound) { // 読んだあとで削除された(D4b-6)
			return nil
		}
		if err != nil {
			return err
		}
		if !canAutoLink(cur.links, link.Ref) || !canAutoLink(cur.links, old) {
			return nil
		}
		if err := tx.WriteItems().Append(id, cur.links.Attach(link.Ref, link.Values)...); err != nil {
			return err
		}
		wrote = true
		if first {
			return nil
		}
		fs, err = fire(tx, id, nil, link, p)
		return err
	})
	if err != nil || !wrote {
		return false, nil, err
	}
	return true, fs, nil
}

// canAutoLink は、links に r と同じ番号のリンク(リポジトリの大文字小文字・種別を問わない)がなく、
// 解除の記録にもないか(D4b-5。Attach は解除の記録を消すので、呼ぶ前に確かめる)。
func canAutoLink(links itemlink.Set, r itemlink.Ref) bool {
	if _, ok := links.Find(r); ok {
		return false
	}
	for _, o := range links.Off {
		if o.Same(r) {
			return false
		}
	}
	return true
}
