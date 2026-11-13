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
	pr, err := port.LiveProject(r, p)
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

// notFound は、いまない値を指した断りを ErrNotFound にする。
func notFound(err error) error {
	if errors.Is(err, projectrule.ErrUnknown) {
		return port.ErrNotFound
	}
	return err
}

// edit は、build が作る行を 1 回で保存する。行がなければ保存しない。
func (s Service) edit(ctx context.Context, store port.StoreID, p project.ID, build func(state) (plan, error)) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var pl plan
	var ws workspace.ID
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		st, err := load(r, p)
		if err != nil {
			return err
		}
		ws = st.ws
		pl, err = build(st)
		return notFound(err)
	})
	if err != nil || len(pl.drafts) == 0 {
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: change.User(me), Summary: pl.summary}, func(tx port.Tx) error {
		st, err := load(tx, p)
		if err != nil {
			return err
		}
		now, err := build(st)
		if err != nil {
			return notFound(err)
		}
		return tx.WriteWorkspaces().Append(ws, now.drafts...)
	})
}
