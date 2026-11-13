package github

import (
	"context"
	"errors"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Links は、アイテムの PR・Issue のリンクの操作(07「PR と Issue のリンク」)が使う口。1 ファイル 1 操作。
// GitHub への問い合わせは、その Store に割り当てたアカウントで行う(Connector.TokenFor)。
type Links struct {
	Stores    port.Stores
	Identity  port.Identity
	Connector Connector
	Checks    port.Checks
	Events    port.Events
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

func readItem(r port.Reader, id item.ID) (itemView, error) {
	it, err := r.Items().Get(id)
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
	p, err := r.Projects().Get(it.Place.Project)
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
	tok, a, err := l.Connector.TokenFor(ctx, store, repo)
	if err != nil {
		return port.Found{}, err
	}
	f, err := l.Connector.GitHub.Lookup(ctx, tok, repo, number)
	if ghcall.Is(err, ghcall.TokenRejected) {
		l.Connector.Rejected(a.ID)
		return port.Found{}, authError(a)
	}
	return f, err
}

func (l Links) now() time.Time { return l.Connector.Clock.Now() }

// saveChecks は、このマシンが読んだチェックの結果を置く。変わったら true(D3-7)。
func (l Links) saveChecks(ctx context.Context, store port.StoreID, r itemlink.Ref, c itemlink.Checks) (bool, error) {
	return l.Checks.Save(ctx, store, r.Key(), c)
}

// issueOwner は、ref の Issue にリンクしているアイテム(store の self 以外)を、このマシンのすべての Store から探し、
// その画面の ID を返す(07「追加」。D3-15)。self を除くのは store の中だけ(別の Store に同じ内部 ID があっても別のアイテム)。
func (l Links) issueOwner(ctx context.Context, store port.StoreID, ref itemlink.Ref, self item.ID) (string, bool, error) {
	infos, err := l.Connector.Stores.List(ctx)
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
func (l Links) failText(err error) string {
	var r *refusal.Refusal
	if !errors.As(l.refused(err), &r) {
		errors.As(l.refused(&ghcall.Failure{Kind: ghcall.Unreachable}), &r)
	}
	return "状態を更新できませんでした: " + r.Message
}
