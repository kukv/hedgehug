package githublink

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// autoLink は、PR pull を、Store の items が指すアイテムに自動でリンクする(D4b-5〜7)。Closed の PR は何もしない。
// repo は GitHub の nameWithOwner(リンクはこの名前で書く)。
// アイテムごとに 1 回の保存(実行者「GitHub」)。first なら(そのリポジトリを初めて探した)きっかけを起こさない。
// p が nil でなければ、同じ読み取りのきっかけの重なりを除く。どれかのアイテムにリンクを書いたら true。
func (l Links) autoLink(ctx context.Context, store port.StoreID, repo string, pull port.Pull, items storeItems, first bool, p *pass) (bool, error) {
	if pull.State == itemlink.Closed {
		return false, nil
	}
	link := itemlink.Link{Ref: itemlink.Ref{Kind: itemlink.PR, Repo: repo, Number: pull.Number}, Values: pull.Values}
	linked := false
	var errs []error // 保存のあとの通知の失敗(ほかのアイテムの保存は続ける)
	for _, id := range items.targets(pull) {
		ok, fs, err := l.saveAutoLink(ctx, store, id, items.live[id], link, items.renames, first, p)
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
// 保存の中で読み直し、削除済み・同じ PR のリンクがある・解除の記録にあるときは書かず false。同じ PR かは、
// リポジトリを改名の対応 renames でいまの名前に引き直して比べる。
func (l Links) saveAutoLink(ctx context.Context, store port.StoreID, id item.ID, display string, link itemlink.Link, renames map[string]string, first bool, p *pass) (bool, []fired, error) {
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
		if !canAutoLink(cur.links, link.Ref, renames) {
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

// canAutoLink は、links に r と同じ PR のリンク(種別を問わない)がなく、解除の記録にもないか(D4b-5。Attach は
// 解除の記録を消すので、呼ぶ前に確かめる)。リポジトリは両方とも renames でいまの名前に引き直して比べる
// (改名の前の名前で解除した PR・Merged のリンクに当てる。D4b 最終レビュー R-1)。
func canAutoLink(links itemlink.Set, r itemlink.Ref, renames map[string]string) bool {
	repo := currentName(renames, r.Repo)
	same := func(o itemlink.Ref) bool { return o.Number == r.Number && currentName(renames, o.Repo) == repo }
	return !slices.ContainsFunc(links.Links, func(l itemlink.Link) bool { return same(l.Ref) }) && !slices.ContainsFunc(links.Off, same)
}

// currentName は、repo を改名の対応 renames で最後まで辿った、いまの名前(小文字)。輪になっていれば、
// 一度通った名前に戻ったところで止める。
func currentName(renames map[string]string, repo string) string {
	name := strings.ToLower(repo)
	seen := map[string]bool{name: true}
	for {
		next, ok := renames[name]
		if !ok || seen[next] {
			return name
		}
		seen[next] = true
		name = next
	}
}
