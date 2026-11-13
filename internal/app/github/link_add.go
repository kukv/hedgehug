package github

import (
	"context"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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
	toProject bool
	summary   string
}

// Add は、PR か Issue を GitHub で確かめてリンクする(07「追加」)。確かめる順は 07 の表のとおり。
// ダイアログは閉じ、トーストは出さない(画面)。手でのリンクでは自動化のきっかけを起こさない(D3-13)。
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
	return l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := readItem(tx, id)
		if err != nil {
			return err
		}
		if err := checkItem(tx, cur, plan.ref, id); err != nil {
			return err
		}
		if err := tx.WriteItems().Append(id, cur.links.Attach(plan.ref, plan.values)...); err != nil {
			return err
		}
		if plan.toProject && project.IndexRepo(cur.project.Repos, plan.ref.Repo) < 0 {
			return tx.WriteProjects().Append(cur.project.ID, change.NewAdd("repos", plan.ref.Repo))
		}
		return nil
	})
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
	f, err := l.lookup(ctx, store, repo, n)
	if err != nil {
		return addPlan{}, l.refused(err)
	}
	ref := itemlink.Ref{Kind: f.Kind, Repo: f.Repo, Number: f.Number}
	if err := l.checkFound(ctx, v, ref, in.Kind, id); err != nil {
		return addPlan{}, err
	}
	if _, err := l.saveChecks(ctx, store, ref, f.Checks); err != nil {
		return addPlan{}, err
	}
	toProject := in.AddToProject && project.IndexRepo(v.project.Repos, f.Repo) < 0
	return addPlan{ref: ref, values: f.Values, toProject: toProject, summary: itemlink.AddSummary(v.display, ref, toProject)}, nil
}

// checkFound は、GitHub から読んだあとの検査(種別の違い・重なり・Issue の 1 対 1)。
func (l Links) checkFound(ctx context.Context, v itemView, ref itemlink.Ref, want itemlink.Kind, id item.ID) error {
	if ref.Kind != want {
		return refusal.New(itemlink.MsgWrongKind(ref, want))
	}
	if _, ok := v.links.Find(ref); ok {
		return refusal.New(itemlink.MsgAlready)
	}
	if ref.Kind != itemlink.Issue {
		return nil
	}
	owner, taken, err := l.issueOwner(ctx, ref, id)
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
