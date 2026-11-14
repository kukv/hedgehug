// Package projectrules は、プロジェクト別のルール(使うステータス・ワークフローの制約・テンプレート・
// ルールの動かす動かさない)を編集する操作(仕様 03「プロジェクト別のルール」)を持つ。
package projectrules

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Service は、プロジェクト別のルールの操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
}

// plan は、1 つの操作の保存の中身。summary は活動の文、drafts は書く行。
type plan struct {
	summary string
	drafts  []change.Draft
}

// state は、操作が見る、いまのプロジェクトとそのルール。
type state struct {
	project project.Project
	ws      workspace.ID
	set     projectrule.Set
	st      workspace.Settings
	rules   []automation.Rule // ワークスペースのログのルールすべて(専用ルールを含む)
}

// mine は、このプロジェクトのルール。
func (s state) mine() projectrule.Rules { return s.set.Of(s.project.ID) }

// own は、このプロジェクトだけの自動化のルール。
func (s state) own() []automation.Rule {
	var out []automation.Rule
	for _, r := range s.rules {
		if r.Scope == s.project.ID {
			out = append(out, r)
		}
	}
	return out
}

func load(r port.Reader, p project.ID) (state, error) {
	pr, err := projectToSave(r, p)
	if err != nil {
		return state{}, err
	}
	ws := pr.Place.Workspace
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return state{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return state{}, err
	}
	return state{project: pr, ws: ws, set: projectrule.FromLog(log), st: st, rules: automation.FromLog(log)}, nil
}

// projectToSave は、ルールを保存できるプロジェクト。削除されていないものと、ワークスペースの削除で一緒に削除された
// もの(保存でワークスペースの削除を取り消し、プロジェクトも一緒に戻る。決定 R11)。自分で削除されたものは ErrNotFound。
func projectToSave(r port.Reader, p project.ID) (project.Project, error) {
	pr, err := r.Projects().Get(p)
	if err != nil || pr.Deleted == nil {
		return pr, err
	}
	ok, err := port.DeletedWithWorkspace(r, pr)
	if err != nil {
		return project.Project{}, err
	}
	if !ok {
		return project.Project{}, port.ErrNotFound
	}
	return pr, nil
}

// reviveWorkspace は、書く Tx の中で、プロジェクト p がワークスペースの削除で一緒に削除されていれば、そのワークスペースの
// 削除を取り消す(プロジェクトも一緒に戻る)。取り消したら true。
func reviveWorkspace(ctx context.Context, tx port.Tx, p project.ID) (bool, error) {
	pr, err := tx.Projects().Get(p)
	if err != nil {
		return false, err
	}
	ok, err := port.DeletedWithWorkspace(tx, pr)
	if err != nil || !ok {
		return false, err
	}
	return port.ReviveWorkspace(ctx, tx, pr.Place.Workspace, "", "")
}

// notFound は、いまない値を指した断りを ErrNotFound にする。
func notFound(err error) error {
	if errors.Is(err, projectrule.ErrUnknown) {
		return port.ErrNotFound
	}
	return err
}

// edit は、build が作る行を 1 回で保存する。行がなければ保存しない。ワークスペースが別の画面で削除されていて、
// プロジェクトも一緒に削除されていれば、ワークスペースの削除を取り消す。そのとき行がなければ ErrNotFound(決定 R11)。
func (s Service) edit(ctx context.Context, store port.StoreID, p project.ID, build func(state) (plan, error)) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var pl plan
	var ws workspace.ID
	var gone bool
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		st, err := load(r, p)
		if err != nil {
			return err
		}
		ws, gone = st.ws, st.project.Deleted != nil
		pl, err = build(st)
		return notFound(err)
	})
	if err != nil || len(pl.drafts) == 0 {
		if err == nil && gone {
			return port.ErrNotFound
		}
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: change.User(me), Summary: pl.summary}, func(tx port.Tx) error {
		return write(ctx, tx, p, ws, build)
	})
}

// write は、edit の書く Tx の中身。ワークスペースの削除を取り消してから、読み直した状態から build で行を書く。
func write(ctx context.Context, tx port.Tx, p project.ID, ws workspace.ID, build func(state) (plan, error)) error {
	revived, err := reviveWorkspace(ctx, tx, p)
	if err != nil {
		return err
	}
	st, err := load(tx, p)
	if err != nil {
		return err
	}
	now, err := build(st)
	if err != nil {
		return notFound(err)
	}
	if revived && len(now.drafts) == 0 {
		return port.ErrNotFound
	}
	if len(now.drafts) == 0 {
		// 読んでから書くまでの間に、ほかの保存で同じ状態になった
		return nil
	}
	return tx.WriteWorkspaces().Append(ws, now.drafts...)
}
