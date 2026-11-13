package projectrules

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

// SetStatusUsed は、ステータスをプロジェクトで使う・使わないにする(B5a-12)。
func (s Service) SetStatusUsed(ctx context.Context, store port.StoreID, p project.ID, status change.ID, used bool) error {
	return s.edit(ctx, store, p, func(st state) (plan, error) {
		var own []projectrule.ScopedRule
		for _, r := range st.own() {
			own = append(own, projectrule.ScopedRule{Name: r.Name, Statuses: automation.Statuses(r)})
		}
		ds, err := projectrule.StatusUseDrafts(st.st, st.mine(), p, status, used, own)
		return plan{fmt.Sprintf("プロジェクト「%s」の使うステータスを変更", st.project.Title), ds}, err
	})
}

// SetTransition は、プロジェクトの遷移 from → to を許す・許さないにする(B5a-13)。
func (s Service) SetTransition(ctx context.Context, store port.StoreID, p project.ID, from, to change.ID, allowed bool) error {
	return s.edit(ctx, store, p, func(st state) (plan, error) {
		ds, err := projectrule.WorkflowDrafts(st.st, st.mine(), p, from, to, allowed)
		return plan{fmt.Sprintf("プロジェクト「%s」のワークフローの制約を変更", st.project.Title), ds}, err
	})
}

// SetTemplateUsed は、テンプレートをプロジェクトで使う・使わないにする。
func (s Service) SetTemplateUsed(ctx context.Context, store port.StoreID, p project.ID, t change.ID, used bool) error {
	return s.edit(ctx, store, p, func(st state) (plan, error) {
		ds, err := projectrule.TemplateUseDrafts(st.st, st.mine(), p, t, used)
		return plan{fmt.Sprintf("プロジェクト「%s」の使うテンプレートを変更", st.project.Title), ds}, err
	})
}

// SetTemplateDefault は、種類 kind の既定のテンプレートを value にする。
func (s Service) SetTemplateDefault(ctx context.Context, store port.StoreID, p project.ID, kind, value string) error {
	if !slices.Contains([]item.Kind{item.Epic, item.Story, item.Task, item.Bug, item.Step}, item.Kind(kind)) {
		return port.ErrNotFound
	}
	return s.edit(ctx, store, p, func(st state) (plan, error) {
		d, ok, err := projectrule.TemplateDefaultDraft(st.st, st.mine(), p, kind, value)
		pl := plan{summary: fmt.Sprintf("プロジェクト「%s」の%sの既定のテンプレートを変更", st.project.Title, kind)}
		if ok {
			pl.drafts = []change.Draft{d}
		}
		return pl, err
	})
}

// SetRuleRuns は、ワークスペース全体のルール rule をプロジェクトで動かす・動かさないにする。
func (s Service) SetRuleRuns(ctx context.Context, store port.StoreID, p project.ID, rule change.ID, runs bool) error {
	return s.edit(ctx, store, p, func(st state) (plan, error) {
		i := slices.IndexFunc(st.rules, func(r automation.Rule) bool { return r.ID == rule && r.Scope == "" })
		if i < 0 {
			return plan{}, port.ErrNotFound
		}
		verb := "動かさない"
		if runs {
			verb = "動かす"
		}
		pl := plan{summary: fmt.Sprintf("プロジェクト「%s」で自動化「%s」を%sようにする", st.project.Title, st.rules[i].Name, verb)}
		if d, ok := projectrule.RuleRunsDraft(st.mine(), p, rule, runs); ok {
			pl.drafts = []change.Draft{d}
		}
		return pl, nil
	})
}
