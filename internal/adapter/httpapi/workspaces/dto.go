package workspaces

import (
	"encoding/json"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 本文の形。domain の型をそのまま出さず、API の名前(小文字の始まり)にそろえる。
// 型の名前は OpenAPI のスキーマ名になるので、機能の名前を付けて公開する。
// 埋め込む型も公開名にする(huma は公開されていない埋め込みを読み飛ばす)。

// WorkspaceSummary は、一覧と作成が返すワークスペース。
type WorkspaceSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

// WorkspaceValue は、ステータス・優先度・システムの値。
type WorkspaceValue struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color" enum:"gray,slate,brown,red,rose,pink,magenta,purple,indigo,blue,sky,cyan,teal,green,olive,amber,yellow,orange"`
	Category string `json:"category,omitempty" enum:"todo,doing,done,canceled" doc:"ステータスの分類(todo・doing・done・canceled)"`
}

// WorkspaceProperty は、プロパティの定義。
type WorkspaceProperty struct {
	ID      string           `json:"id"`
	Key     string           `json:"key"`
	Name    string           `json:"name"`
	Type    string           `json:"type"`
	Builtin bool             `json:"builtin"`
	Kinds   []string         `json:"kinds" nullable:"false"`
	Hidden  bool             `json:"hidden"`
	Choices []WorkspaceValue `json:"choices" nullable:"false" doc:"選択肢(単一選択・複数選択だけ。ほかは空)"`
}

// WorkspaceSettings は、ワークスペースの設定。
type WorkspaceSettings struct {
	Properties       []WorkspaceProperty `json:"properties" nullable:"false"`
	Statuses         []WorkspaceValue    `json:"statuses" nullable:"false"`
	Priorities       []WorkspaceValue    `json:"priorities" nullable:"false"`
	Systems          []WorkspaceValue    `json:"systems" nullable:"false"`
	InitialStatus    string              `json:"initialStatus"`
	InitialPriority  string              `json:"initialPriority"`
	CategoryDefaults map[string]string   `json:"categoryDefaults"`
	Workflow         map[string][]string `json:"workflow" doc:"From のステータス ID → 許可する To(ステータスの並び)。制限のない From は入れない"`
	Templates        []WorkspaceTemplate `json:"templates" nullable:"false"`
	TemplateDefaults map[string]string   `json:"templateDefaults" doc:"種類 → 既定のテンプレート ID。なしの種類は入れない"`
}

// WorkspaceTemplateStep は、テンプレートの Step。
type WorkspaceTemplateStep struct {
	ID       string `json:"id" doc:"新しく足すときは空"`
	Title    string `json:"title"`
	Assignee string `json:"assignee" doc:"@parent(親と同じ)・@none(未割り当て)・メンバーのログイン名"`
}

// WorkspaceTemplateChild は、テンプレートの子(Epic のときだけ)。
type WorkspaceTemplateChild struct {
	ID          string                  `json:"id" doc:"新しく足すときは空"`
	Kind        string                  `json:"kind" enum:"Story,Task,Bug"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Values      map[string]any          `json:"values" doc:"プロパティの ID → 値。null は指定しない"`
	Steps       []WorkspaceTemplateStep `json:"steps" nullable:"false"`
}

// WorkspaceTemplateInput は、テンプレートの作成・編集の本文。編集で送らなかった値は残る。
type WorkspaceTemplateInput struct {
	Name        string                   `json:"name"`
	Kind        string                   `json:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Title       string                   `json:"title"`
	Description string                   `json:"description"`
	Values      map[string]any           `json:"values" doc:"プロパティの ID → 値。null は指定しない"`
	Children    []WorkspaceTemplateChild `json:"children" nullable:"false"`
	Steps       []WorkspaceTemplateStep  `json:"steps" nullable:"false"`
}

// WorkspaceTemplate は、テンプレート。
type WorkspaceTemplate struct {
	ID string `json:"id"`
	WorkspaceTemplateInput
}

// WorkspaceDetail は、詳細が返すワークスペースと設定。
type WorkspaceDetail struct {
	WorkspaceSummary
	OldPrefixes []string          `json:"oldPrefixes" nullable:"false"`
	Settings    WorkspaceSettings `json:"settings"`
}

func toSummary(w workspace.Workspace) WorkspaceSummary {
	return WorkspaceSummary{ID: string(w.ID), Name: w.Name, Prefix: w.Prefix}
}

func toValues(vs []workspace.Value) []WorkspaceValue {
	out := make([]WorkspaceValue, 0, len(vs))
	for _, v := range vs {
		out = append(out, WorkspaceValue{ID: string(v.ID), Name: v.Name, Color: string(v.Color), Category: string(v.Category)})
	}
	return out
}

func toSettings(s workspace.Settings) WorkspaceSettings {
	out := WorkspaceSettings{
		Statuses: toValues(s.Statuses), Priorities: toValues(s.Priorities), Systems: toValues(s.Systems),
		InitialStatus: string(s.InitialStatus), InitialPriority: string(s.InitialPriority),
		CategoryDefaults: map[string]string{}, Properties: []WorkspaceProperty{},
		Workflow: map[string][]string{}, Templates: []WorkspaceTemplate{}, TemplateDefaults: map[string]string{},
	}
	for c, id := range s.CategoryDefaults {
		out.CategoryDefaults[string(c)] = string(id)
	}
	for _, p := range s.Properties {
		out.Properties = append(out.Properties, toProperty(p))
	}
	for _, v := range s.Statuses {
		if to, ok := s.Allowed(v.ID); ok {
			ids := make([]string, 0, len(to))
			for _, id := range to {
				ids = append(ids, string(id))
			}
			out.Workflow[string(v.ID)] = ids
		}
	}
	for _, t := range s.Templates {
		out.Templates = append(out.Templates, toTemplate(t))
	}
	for k, id := range s.TemplateDefaults {
		out.TemplateDefaults[k] = string(id)
	}
	return out
}

// fromValues は、本文の値を JSON の値にする(nil は null。「指定しない」に戻す)。
func fromValues(vs map[string]any) map[change.ID]json.RawMessage {
	out := make(map[change.ID]json.RawMessage, len(vs))
	for k, v := range vs {
		b, _ := json.Marshal(v)
		out[change.ID(k)] = b
	}
	return out
}

// toValuesMap は、JSON の値を本文の値にする。
func toValuesMap(vs map[change.ID]json.RawMessage) map[string]any {
	out := make(map[string]any, len(vs))
	for k, raw := range vs {
		var v any
		_ = json.Unmarshal(raw, &v)
		out[string(k)] = v
	}
	return out
}

func toTemplateSteps(ss []workspace.TemplateStep) []WorkspaceTemplateStep {
	out := make([]WorkspaceTemplateStep, 0, len(ss))
	for _, s := range ss {
		out = append(out, WorkspaceTemplateStep{ID: string(s.ID), Title: s.Title, Assignee: s.Assignee})
	}
	return out
}

func fromTemplateSteps(ss []WorkspaceTemplateStep) []workspace.TemplateStep {
	out := make([]workspace.TemplateStep, 0, len(ss))
	for _, s := range ss {
		out = append(out, workspace.TemplateStep{ID: change.ID(s.ID), Title: s.Title, Assignee: s.Assignee})
	}
	return out
}

func toTemplate(t workspace.Template) WorkspaceTemplate {
	children := make([]WorkspaceTemplateChild, 0, len(t.Children))
	for _, c := range t.Children {
		children = append(children, WorkspaceTemplateChild{
			ID: string(c.ID), Kind: c.Kind, Title: c.Title, Description: c.Description,
			Values: toValuesMap(c.Values), Steps: toTemplateSteps(c.Steps),
		})
	}
	return WorkspaceTemplate{ID: string(t.ID), WorkspaceTemplateInput: WorkspaceTemplateInput{
		Name: t.Name, Kind: t.Kind, Title: t.Title, Description: t.Description,
		Values: toValuesMap(t.Values), Children: children, Steps: toTemplateSteps(t.Steps),
	}}
}

func fromTemplate(in WorkspaceTemplateInput) workspace.Template {
	children := make([]workspace.TemplateChild, 0, len(in.Children))
	for _, c := range in.Children {
		children = append(children, workspace.TemplateChild{
			ID: change.ID(c.ID), Kind: c.Kind, Title: c.Title, Description: c.Description,
			Values: fromValues(c.Values), Steps: fromTemplateSteps(c.Steps),
		})
	}
	return workspace.Template{
		Name: in.Name, Kind: in.Kind, Title: in.Title, Description: in.Description,
		Values: fromValues(in.Values), Children: children, Steps: fromTemplateSteps(in.Steps),
	}
}

func toProperty(p workspace.Property) WorkspaceProperty {
	kinds := p.Kinds
	if kinds == nil {
		kinds = []string{}
	}
	return WorkspaceProperty{
		ID: string(p.ID), Key: p.Key, Name: p.Name, Type: string(p.Type),
		Builtin: p.Builtin, Kinds: kinds, Hidden: p.Hidden, Choices: toValues(p.Choices),
	}
}
