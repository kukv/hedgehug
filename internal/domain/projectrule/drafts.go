package projectrule

import (
	"errors"
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ErrUnknown は、いまない値(ステータス・テンプレート)や、意味のない組(同じステータス同士)を指したとき。
// 呼ぶ側は 404 にする。
var ErrUnknown = errors.New("projectrule: unknown value")

// ScopedRule は、このプロジェクトだけの自動化のルールのうち、断りの判定に要るもの。
// 専用ルールは automation に属すので、呼ぶ側がこの形にして渡す。
type ScopedRule struct {
	Name     string
	Statuses []change.ID // ルールが使うステータス
}

// Section は、「ワークスペースの設定に戻す」の区画(B5a-14)。
type Section string

const (
	SectionStatuses   Section = "statuses"
	SectionWorkflow   Section = "workflow"
	SectionTemplates  Section = "templates"
	SectionAutomation Section = "automation"
)

// Label は、区画の画面の名前。
func (s Section) Label() string {
	switch s {
	case SectionStatuses:
		return "使うステータス"
	case SectionWorkflow:
		return "ワークフローの制約"
	case SectionTemplates:
		return "テンプレート"
	case SectionAutomation:
		return "自動化"
	}
	return string(s)
}

func hasStatus(s workspace.Settings, id change.ID) bool {
	return slices.ContainsFunc(s.Statuses, func(v workspace.Value) bool { return v.ID == id })
}

// StatusUseDrafts は、ステータス status を使う(used)・使わないにする操作の行(B5a-12)。使わないにする
// ときだけ、初期ステータス・分類の既定・own(専用ルール)が使うものを断る。すでにその状態なら行なし。
func StatusUseDrafts(s workspace.Settings, r Rules, p project.ID, status change.ID, used bool, own []ScopedRule) ([]change.Draft, error) {
	hidden := slices.Contains(r.Hidden, status)
	if !hasStatus(s, status) && (!used || !hidden) {
		return nil, ErrUnknown
	}
	if hidden == !used {
		return nil, nil
	}
	if used {
		return []change.Draft{change.NewRemove(HiddenField(p), status)}, nil
	}
	if err := checkCanHide(s, status, own); err != nil {
		return nil, err
	}
	return []change.Draft{change.NewAdd(HiddenField(p), status)}, nil
}

// IsFixedStatus は、使わないことにできないステータス(初期ステータスか、分類ごとの既定)か。
// checkCanHide の断りと、プロジェクトの移動での引き継ぎ(transfer.CarryRules)が同じ判定を使う。
func IsFixedStatus(s workspace.Settings, status change.ID) bool {
	_, isDefault := s.DefaultCategory(status)
	return s.InitialStatus == status || isDefault
}

func checkCanHide(s workspace.Settings, status change.ID, own []ScopedRule) error {
	if IsFixedStatus(s, status) {
		initial := s.InitialStatus == status
		cat, isDefault := s.DefaultCategory(status)
		label := workspace.CategoryName(cat)
		switch {
		case initial && isDefault:
			return refusal.New("初期ステータスで、" + label + "の既定でもあるため外せません")
		case initial:
			return refusal.New("初期ステータスのため外せません")
		default:
			return refusal.New(label + "の既定のため外せません")
		}
	}
	for _, rule := range own {
		if slices.Contains(rule.Statuses, status) {
			return refusal.New("「" + workspace.StatusName(s, status) + "」はこのプロジェクトだけのルール「" + rule.Name +
				"」が使っているため、使わないことにできません。先にルールを変えてください")
		}
	}
	return nil
}

// WorkflowDrafts は、from から to の遷移のセルにチェックを付ける(allowed)・外す操作の行(B5a-13)。
// ワークスペースの制約が許さないセルへのチェックは断る。すでにその状態なら行なし。
func WorkflowDrafts(s workspace.Settings, r Rules, p project.ID, from, to change.ID, allowed bool) ([]change.Draft, error) {
	if from == to || !hasStatus(s, from) || !hasStatus(s, to) {
		return nil, ErrUnknown
	}
	if slices.Contains(r.Workflow[from], to) == allowed {
		return nil, nil
	}
	if !allowed {
		return []change.Draft{change.NewRemove(WorkflowField(p, from), to)}, nil
	}
	if ws, restricted := s.Allowed(from); restricted && !slices.Contains(ws, to) {
		return nil, refusal.New("ワークスペースの制約で変更できません")
	}
	return []change.Draft{change.NewAdd(WorkflowField(p, from), to)}, nil
}

// TemplateUseDrafts は、テンプレート t をプロジェクトで使う(used)・使わないにする操作の行。
// すでにその状態なら行なし。
func TemplateUseDrafts(s workspace.Settings, r Rules, p project.ID, t change.ID, used bool) ([]change.Draft, error) {
	listed := slices.Contains(r.Templates, t)
	if _, ok := s.Template(t); !ok && (used || !listed) {
		return nil, ErrUnknown
	}
	switch {
	case listed == used:
		return nil, nil
	case used:
		return []change.Draft{change.NewAdd(TemplatesField(p), t)}, nil
	}
	return []change.Draft{change.NewRemove(TemplatesField(p), t)}, nil
}

// TemplateDefaultDraft は、種類 kind の既定のテンプレートを value にする操作の行。value は ""(ワークスペース
// と同じ)・None・プロジェクトで使えるテンプレート ID。変わらなければ false。
func TemplateDefaultDraft(s workspace.Settings, r Rules, p project.ID, kind, value string) (change.Draft, bool, error) {
	old := r.TemplateDefaults[kind]
	if value != "" && value != None {
		usable := UsableTemplates(s, r, kind)
		if !slices.ContainsFunc(usable, func(t workspace.Template) bool { return t.ID == change.ID(value) }) {
			return change.Draft{}, false, ErrUnknown
		}
	}
	if old == value {
		return change.Draft{}, false, nil
	}
	return change.NewSet(TemplateDefaultField(p, kind), nilIfEmpty(old), nilIfEmpty(value)), true, nil
}

func nilIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// RuleRunsDraft は、ワークスペースのルール rule をこのプロジェクトで動かす(runs)・外す操作の行。
// すでにその状態なら false。
func RuleRunsDraft(r Rules, p project.ID, rule change.ID, runs bool) (change.Draft, bool) {
	disabled := slices.Contains(r.Disabled, rule)
	switch {
	case disabled == !runs:
		return change.Draft{}, false
	case runs:
		return change.NewRemove(DisabledField(p), rule), true
	}
	return change.NewAdd(DisabledField(p), rule), true
}

// ResetDrafts は、区画 sec をワークスペースの設定に戻す行(B5a-14)。自動化の専用ルールの削除は
// 呼ぶ側が足す。
func ResetDrafts(r Rules, p project.ID, sec Section) []change.Draft {
	var ds []change.Draft
	switch sec {
	case SectionStatuses:
		for _, id := range r.Hidden {
			ds = append(ds, change.NewRemove(HiddenField(p), id))
		}
	case SectionWorkflow:
		for _, from := range slices.Sorted(maps.Keys(r.Workflow)) {
			for _, to := range r.Workflow[from] {
				ds = append(ds, change.NewRemove(WorkflowField(p, from), to))
			}
		}
	case SectionTemplates:
		for _, id := range r.Templates {
			ds = append(ds, change.NewRemove(TemplatesField(p), id))
		}
		for _, kind := range slices.Sorted(maps.Keys(r.TemplateDefaults)) {
			ds = append(ds, change.NewSet(TemplateDefaultField(p, kind), r.TemplateDefaults[kind], nil))
		}
	case SectionAutomation:
		for _, id := range r.Disabled {
			ds = append(ds, change.NewRemove(DisabledField(p), id))
		}
	}
	return ds
}
