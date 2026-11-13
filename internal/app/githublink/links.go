// Package githublink は、アイテムの PR・Issue のリンクの操作(仕様 07「PR と Issue のリンク」)を持つ。
package githublink

import (
	"context"
	"errors"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Links は、アイテムの PR・Issue のリンクの操作(07「PR と Issue のリンク」)が使う口。1 ファイル 1 操作。
// GitHub への問い合わせは、その Store に割り当てたアカウントで行う(Tokens.TokenFor。D4a-2)。
type Links struct {
	Stores   port.Stores
	Identity port.Identity
	Tokens   port.GitHubTokens
	GitHub   port.GitHub
	Accounts port.Accounts    // skipRefresh(このマシンにアカウントが 1 つもない)
	StoreGH  port.StoreGitHub // 閲覧のみの判定と、Issue の 1 対 1 の Store の一覧
	Checks   port.Checks
	Reads    port.GitHubReads // 取り直しの失敗の文言(D4a-11)
	Scans    port.PRScans     // PR を探した記録(D4b-4)
	Missing  port.AppMissing  // GitHub App 未導入の組織の記録(D4b-12)
	Inbox    port.Inbox       // 動いたルールの通知(D4a-17)
	Events   port.Events
	Clock    port.Clock
}

// LinkRow は、リンクの一覧の 1 行。
type LinkRow struct {
	itemlink.Link
	Checks itemlink.Checks
	Error  string
}

// itemView は、リンクの操作が読むアイテムまわりの値。
type itemView struct {
	item    item.Item
	display string // 画面の ID(HH-2)
	links   itemlink.Set
	project project.Project
}

// readItem は、削除済みでないアイテムと、そのプロジェクトを読む。どちらかが削除済みなら port.ErrNotFound(A4b-4)。
func readItem(r port.Reader, id item.ID) (itemView, error) {
	it, err := port.LiveItem(r, id)
	if err != nil {
		return itemView{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
	if err != nil {
		return itemView{}, err
	}
	ws, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return itemView{}, err
	}
	p, err := port.LiveProject(r, it.Place.Project)
	if err != nil {
		return itemView{}, err
	}
	return itemView{item: it, display: it.DisplayID(ws.Prefix), links: itemlink.FromLog(log), project: p}, nil
}

func (l Links) view(ctx context.Context, store port.StoreID, id item.ID) (itemView, error) {
	var v itemView
	err := l.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		v, err = readItem(r, id)
		return err
	})
	return v, err
}

// lookup は、Store のアカウントで repo の number を読む。トークンが拒否されたら認証エラーにする(07「状態」)。
func (l Links) lookup(ctx context.Context, store port.StoreID, repo string, number int) (port.Found, error) {
	var found port.Found
	err := l.call(ctx, store, repo, func(tok string) (string, error) {
		var err error
		found, err = l.GitHub.Lookup(ctx, tok, repo, number)
		return found.Repo, err
	})
	if err != nil {
		return port.Found{}, err
	}
	return found, nil
}

func (l Links) now() time.Time { return l.Clock.Now() }

// authError は、a の認証エラーの失敗(07「状態」)。
func authError(a account.Account) error {
	return &ghcall.Failure{Kind: ghcall.AuthError, Name: a.Name, Login: a.Login}
}

// saveChecks は、このマシンが読んだチェックの結果を置く。変わったら true(D3-7)。
func (l Links) saveChecks(ctx context.Context, store port.StoreID, r itemlink.Ref, c itemlink.Checks) (bool, error) {
	return l.Checks.Save(ctx, store, r.Key(), c)
}

// issueOwner は、ref の Issue にリンクしているアイテム(store の self 以外)を、このマシンのすべての Store から探し、
// その画面の ID を返す(07「追加」。D3-15)。self を除くのは store の中だけ(別の Store に同じ内部 ID があっても別のアイテム)。
func (l Links) issueOwner(ctx context.Context, store port.StoreID, ref itemlink.Ref, self item.ID) (string, bool, error) {
	infos, err := l.StoreGH.List(ctx)
	if err != nil {
		return "", false, err
	}
	for _, s := range infos {
		var id string
		var found bool
		skip := item.ID("")
		if s.ID == store {
			skip = self
		}
		err := l.Stores.Read(ctx, s.ID, func(r port.Reader) error {
			var err error
			id, found, err = findIssue(r, ref, skip)
			return err
		})
		if errors.Is(err, port.ErrStoreNotFound) {
			continue
		}
		if err != nil || found {
			return id, found, err
		}
	}
	return "", false, nil
}

// findIssue は、r の Store の、self 以外のアイテムで、ref の Issue にリンクしているものを探す。
// 削除済みのアイテム・プロジェクトも調べる(07「追加」)。
func findIssue(r port.Reader, ref itemlink.Ref, self item.ID) (string, bool, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return "", false, err
	}
	for _, w := range wss {
		ps, err := r.Projects().List(w.ID)
		if err != nil {
			return "", false, err
		}
		for _, p := range ps {
			id, ok, err := findIssueIn(r, p.ID, w.Prefix, ref, self)
			if err != nil || ok {
				return id, ok, err
			}
		}
	}
	return "", false, nil
}

// findIssueIn は、プロジェクトの中のアイテムを見る。
func findIssueIn(r port.Reader, p project.ID, prefix string, ref itemlink.Ref, self item.ID) (string, bool, error) {
	its, err := r.Items().List(p)
	if err != nil {
		return "", false, err
	}
	for _, it := range its {
		if it.ID == self {
			continue
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
		if err != nil {
			return "", false, err
		}
		if l, ok := itemlink.FromLog(log).Find(ref); ok && l.Kind == itemlink.Issue {
			return it.DisplayID(prefix), true, nil
		}
	}
	return "", false, nil
}

// refused は、GitHub の失敗を 07 の文言の断りにする。
func (l Links) refused(err error) error { return ghcall.Refuse(err, l.now()) }

func rowsOf(ls []itemlink.Link, checks map[string]itemlink.Checks, errs map[string]string) []LinkRow {
	out := make([]LinkRow, len(ls))
	for i, l := range ls {
		out[i] = LinkRow{Link: l, Checks: checks[l.Key()], Error: errs[l.Key()]}
	}
	return out
}

// failText は、取り直しの失敗の行の文言(07「表示と更新」)。
func (l Links) failText(err error) string { return refreshText(l.failMessage(err)) }

// refreshText は、07 の表の文 msg を、取り直しの失敗の行の文言にする。
func refreshText(msg string) string { return "状態を更新できませんでした: " + msg }

// failMessage は、GitHub の失敗の 07 の表の文(前置きなし。PR を探した記録に置く)。ghcall の失敗でなければ
// 「接続できません」の文。
func (l Links) failMessage(err error) string {
	var r *refusal.Refusal
	if !errors.As(l.refused(err), &r) {
		errors.As(l.refused(&ghcall.Failure{Kind: ghcall.Unreachable}), &r)
	}
	return r.Message
}
