package githublink

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// AddLinkInput は「PR をリンク」「Issue をリンク」のダイアログの入力。
type AddLinkInput struct {
	Kind         itemlink.Kind
	Repo         string
	Number       string
	AddToProject bool
}

// addPlan は、確かめ終えて保存するもの。
type addPlan struct {
	ref       itemlink.Ref
	values    itemlink.Values
	checks    itemlink.Checks
	toProject bool
	summary   string
}

// Add は、PR か Issue を GitHub で確かめてリンクする(07「追加」)。確かめる順は 07 の表のとおり。
// ダイアログは閉じ、トーストは出さない(画面)。同じ保存の中で自動化のきっかけを起こし(D4a-16。行の by は
// 操作した人)、保存のあとに動いたルールの通知を書く。
// アクティビティの文(「(プロジェクトのリポジトリにも追加)」の有無)は保存の前の値で決まる。保存の中で、
// 同時に誰かがそのリポジトリをプロジェクトに足していたら、断らずに黙って足さない(文はそのまま。この狭い
// 競合は受け入れる)。チェックの結果は、保存できたあとに置く。
func (l Links) Add(ctx context.Context, store port.StoreID, id item.ID, in AddLinkInput) error {
	if in.Kind != itemlink.PR && in.Kind != itemlink.Issue {
		return port.ErrNotFound
	}
	plan, err := l.plan(ctx, store, id, in)
	if err != nil {
		return err
	}
	me, err := l.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: plan.summary}
	var fs []fired
	err = l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		var err error
		fs, err = saveLink(tx, id, plan)
		return err
	})
	if err != nil {
		return err
	}
	// 通知を先に書く(チェックの結果を置くのに失敗しても、保存したルールの通知は失わない)
	nerr := l.notify(ctx, store, fs)
	_, cerr := l.saveChecks(ctx, store, plan.ref, plan.checks)
	// 解除する前に置いた取り直しの失敗の文言を消す(いま GitHub から読めた。D4a-11)
	_, ferr := l.setFailure(ctx, store, plan.ref.Key(), "")
	return errors.Join(nerr, cerr, ferr)
}

// saveLink は、保存 tx の中で確かめ直してリンクを書き、同じ保存の中できっかけを動かす。
func saveLink(tx port.Tx, id item.ID, plan addPlan) ([]fired, error) {
	cur, err := readItem(tx, id)
	if err != nil {
		return nil, err
	}
	if err := checkItem(tx, cur, plan.ref, id); err != nil {
		return nil, err
	}
	if err := tx.WriteItems().Append(id, cur.links.Attach(plan.ref, plan.values)...); err != nil {
		return nil, err
	}
	if plan.toProject && project.IndexRepo(cur.project.Repos, plan.ref.Repo) < 0 {
		if err := tx.WriteProjects().Append(cur.project.ID, change.NewAdd("repos", plan.ref.Repo)); err != nil {
			return nil, err
		}
	}
	return fire(tx, id, nil, itemlink.Link{Ref: plan.ref, Values: plan.values}, nil)
}

// plan は、入力の形・重なり・GitHub・Issue の 1 対 1 を、07 の表の順に確かめる。
func (l Links) plan(ctx context.Context, store port.StoreID, id item.ID, in AddLinkInput) (addPlan, error) {
	repo, err := project.NormalizeRepo(in.Repo)
	if err != nil {
		return addPlan{}, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(in.Number))
	if err != nil || n <= 0 {
		return addPlan{}, refusal.New(itemlink.MsgNumber)
	}
	v, err := l.view(ctx, store, id)
	if err != nil {
		return addPlan{}, err
	}
	if _, ok := v.links.Find(itemlink.Ref{Kind: in.Kind, Repo: repo, Number: n}); ok {
		return addPlan{}, refusal.New(itemlink.MsgAlready)
	}
	if n > math.MaxInt32 { // GitHub の番号の上限を超える: 問い合わせずに見つからないとする
		return addPlan{}, l.refused(&ghcall.Failure{Kind: ghcall.NotFound, Repo: repo, Number: n})
	}
	f, err := l.lookup(ctx, store, repo, n)
	if err != nil {
		return addPlan{}, l.refused(err)
	}
	ref := itemlink.Ref{Kind: f.Kind, Repo: f.Repo, Number: f.Number}
	if err := l.checkFound(ctx, store, v, ref, in.Kind, id); err != nil {
		return addPlan{}, err
	}
	toProject := in.AddToProject && project.IndexRepo(v.project.Repos, f.Repo) < 0
	return addPlan{ref: ref, values: f.Values, checks: f.Checks, toProject: toProject, summary: itemlink.AddSummary(v.display, ref, toProject)}, nil
}

// checkFound は、GitHub から読んだあとの検査(種別の違い・重なり・Issue の 1 対 1)。
func (l Links) checkFound(ctx context.Context, store port.StoreID, v itemView, ref itemlink.Ref, want itemlink.Kind, id item.ID) error {
	if ref.Kind != want {
		return refusal.New(itemlink.MsgWrongKind(ref, want))
	}
	if _, ok := v.links.Find(ref); ok {
		return refusal.New(itemlink.MsgAlready)
	}
	if ref.Kind != itemlink.Issue {
		return nil
	}
	owner, taken, err := l.issueOwner(ctx, store, ref, id)
	if err != nil {
		return err
	}
	if taken {
		return refusal.New(itemlink.MsgIssueTaken(ref, owner))
	}
	if _, ok := v.links.HasIssue(); ok {
		return refusal.New(itemlink.MsgHasIssue)
	}
	return nil
}

// checkItem は、保存の中で、同じ Store のいまの値で重なりと Issue の 1 対 1 を確かめ直す。
func checkItem(tx port.Tx, cur itemView, ref itemlink.Ref, id item.ID) error {
	if _, ok := cur.links.Find(ref); ok {
		return refusal.New(itemlink.MsgAlready)
	}
	if ref.Kind != itemlink.Issue {
		return nil
	}
	owner, taken, err := findIssue(tx, ref, id)
	if err != nil {
		return err
	}
	if taken {
		return refusal.New(itemlink.MsgIssueTaken(ref, owner))
	}
	if _, ok := cur.links.HasIssue(); ok {
		return refusal.New(itemlink.MsgHasIssue)
	}
	return nil
}
