package automations

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Create は、ルールを作る(03「ルールの一覧」の「ルールを作成」)。有効で始まる。in.Scope があれば、そのプロジェクトだけの
// ルールで、プロジェクトはこのワークスペースの削除済みでないもの(B5a-10)か、ワークスペースと一緒に削除されたもの
// (保存でワークスペースと一緒に戻る。決定 R11)。
func (s Service) Create(ctx context.Context, store port.StoreID, ws workspace.ID, in automation.Rule) (automation.Rule, error) {
	in.ID, in.Enabled = s.IDs.New(), true
	in = automation.Normalize(in)
	err := s.edit(ctx, store, ws, "",
		func(r port.Reader, _ []automation.Rule) (string, error) {
			if in.Scope != "" {
				if err := liveScope(r, ws, in.Scope); err != nil {
					return "", err
				}
			}
			return ruleText(r, in.Scope, in.Name, "を作成")
		},
		func(r port.Reader, st workspace.Settings, set projectrule.Set, rules []automation.Rule, ms []string) ([]change.Draft, error) {
			if in.Scope != "" {
				if err := liveScope(r, ws, in.Scope); err != nil {
					return nil, err
				}
			}
			in = canonicalLogin(in, ms)
			if !automation.Valid(st, set, ms, automation.Rule{}, in) {
				return nil, port.ErrNotFound
			}
			if err := automation.Check(rules, in); err != nil {
				return nil, err
			}
			return automation.Drafts(automation.Rule{}, in), nil
		})
	if err != nil {
		return automation.Rule{}, err
	}
	return in, nil
}

// Update は、ルールを直す。有効・無効は変えない(B4-5)。
func (s Service) Update(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, in automation.Rule) error {
	in.ID = id
	in = automation.Normalize(in)
	return s.edit(ctx, store, ws, ruleField(id),
		func(r port.Reader, rules []automation.Rule) (string, error) {
			old, err := find(rules, id)
			if err != nil {
				return "", err
			}
			return ruleText(r, old.Scope, in.Name, "を編集")
		},
		func(_ port.Reader, st workspace.Settings, set projectrule.Set, rules []automation.Rule, ms []string) ([]change.Draft, error) {
			old, err := find(rules, id)
			if err != nil {
				return nil, err
			}
			in.Enabled, in.Scope = old.Enabled, old.Scope // 編集で Scope は変えない(B5a-10)
			in = canonicalLogin(in, ms)
			if !automation.Valid(st, set, ms, old, in) {
				return nil, port.ErrNotFound
			}
			if err := automation.Check(rules, in); err != nil {
				return nil, err
			}
			return automation.Drafts(old, in), nil
		})
}

// Delete は、ルールを消す。
func (s Service) Delete(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.remove(ctx, store, ws,
		func(rd port.Reader, rules []automation.Rule) (string, error) {
			r, err := find(rules, id)
			if err != nil {
				return "", err
			}
			return ruleText(rd, r.Scope, r.Name, "を削除")
		},
		func(_ port.Reader, _ workspace.Settings, set projectrule.Set, rules []automation.Rule, _ []string) ([]change.Draft, error) {
			if _, err := find(rules, id); err != nil {
				return nil, err
			}
			return append([]change.Draft{automation.DeleteDraft(id)}, projectrule.RuleDeleteDrafts(set, id)...), nil
		})
}

// SetEnabled は、スイッチ(03)。ON にするときは、きっかけと同じステータスへのルールを断る。同じ値なら何も保存しない。
func (s Service) SetEnabled(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, on bool) error {
	word := "無効"
	if on {
		word = "有効"
	}
	return s.edit(ctx, store, ws, ruleField(id),
		func(_ port.Reader, rules []automation.Rule) (string, error) {
			r, err := find(rules, id)
			return fmt.Sprintf("自動化「%s」を%sにする", oneLine(r.Name), word), err
		},
		func(_ port.Reader, _ workspace.Settings, _ projectrule.Set, rules []automation.Rule, _ []string) ([]change.Draft, error) {
			old, err := find(rules, id)
			if err != nil {
				return nil, err
			}
			if on && automation.CheckSameStatus(old) != nil {
				return nil, refusal.New(automation.SwitchRefusal)
			}
			n := old
			n.Enabled = on
			return automation.Drafts(old, n), nil
		})
}

// ruleField は、ルール id の項目名の頭(削除の行の項目名。automation.DeleteDraft と同じ)。
func ruleField(id change.ID) string { return automation.DeleteDraft(id).F }

// liveScope は、専用ルールの範囲が、このワークスペースの削除済みでないプロジェクトであることを確かめる。
// ワークスペースと一緒に削除されたプロジェクトは、保存でワークスペースと一緒に戻るので通す(決定 R11)。
func liveScope(r port.Reader, ws workspace.ID, scope project.ID) error {
	p, err := r.Projects().Get(scope)
	if err != nil {
		return err
	}
	if p.Deleted != nil {
		ok, err := port.DeletedWithWorkspace(r, p)
		if err != nil {
			return err
		}
		if !ok {
			return port.ErrNotFound
		}
	}
	if p.Place.Workspace != ws {
		return port.ErrNotFound
	}
	return nil
}

// ruleText は、ルールの操作のアクティビティの文。専用ルールならプロジェクトの名前を添える(03 の表)。
func ruleText(r port.Reader, scope project.ID, name, verb string) (string, error) {
	if scope == "" {
		return fmt.Sprintf("自動化「%s」%s", oneLine(name), verb), nil
	}
	p, err := r.Projects().Get(scope)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("プロジェクト「%s」の自動化「%s」%s", oneLine(p.Title), oneLine(name), verb), nil
}
