package automations

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Create は、ワークスペース全体のルールを作る(03「ルールの一覧」の「ルールを作成」)。有効で始まる。
func (s Service) Create(ctx context.Context, store port.StoreID, ws workspace.ID, in automation.Rule) (automation.Rule, error) {
	in.ID, in.Enabled = s.IDs.New(), true
	in = automation.Normalize(in)
	err := s.edit(ctx, store, ws,
		func([]automation.Rule) (string, error) {
			return fmt.Sprintf("自動化「%s」を作成", oneLine(in.Name)), nil
		},
		func(st workspace.Settings, set projectrule.Set, rules []automation.Rule, ms []string) ([]change.Draft, error) {
			in = canonicalLogin(in, ms)
			if !automation.Valid(st, set, ms, in) {
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
	return s.edit(ctx, store, ws,
		func(rules []automation.Rule) (string, error) {
			if _, err := find(rules, id); err != nil {
				return "", err
			}
			return fmt.Sprintf("自動化「%s」を編集", oneLine(in.Name)), nil
		},
		func(st workspace.Settings, set projectrule.Set, rules []automation.Rule, ms []string) ([]change.Draft, error) {
			old, err := find(rules, id)
			if err != nil {
				return nil, err
			}
			in.Enabled, in.Scope = old.Enabled, old.Scope // 編集で Scope は変えない(B5a-10)
			in = canonicalLogin(in, ms)
			if !automation.Valid(st, set, ms, in) {
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
	return s.edit(ctx, store, ws,
		func(rules []automation.Rule) (string, error) {
			r, err := find(rules, id)
			return fmt.Sprintf("自動化「%s」を削除", oneLine(r.Name)), err
		},
		func(_ workspace.Settings, _ projectrule.Set, rules []automation.Rule, _ []string) ([]change.Draft, error) {
			if _, err := find(rules, id); err != nil {
				return nil, err
			}
			return []change.Draft{automation.DeleteDraft(id)}, nil
		})
}

// SetEnabled は、スイッチ(03)。ON にするときは、きっかけと同じステータスへのルールを断る。同じ値なら何も保存しない。
func (s Service) SetEnabled(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, on bool) error {
	word := "無効"
	if on {
		word = "有効"
	}
	return s.edit(ctx, store, ws,
		func(rules []automation.Rule) (string, error) {
			r, err := find(rules, id)
			return fmt.Sprintf("自動化「%s」を%sにする", oneLine(r.Name), word), err
		},
		func(_ workspace.Settings, _ projectrule.Set, rules []automation.Rule, _ []string) ([]change.Draft, error) {
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
