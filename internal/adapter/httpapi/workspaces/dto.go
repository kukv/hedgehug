package workspaces

import "github.com/kukv/hedgehug/internal/domain/workspace"

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
	ID      string   `json:"id"`
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Builtin bool     `json:"builtin"`
	Kinds   []string `json:"kinds" nullable:"false"`
	Hidden  bool     `json:"hidden"`
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
	}
	for c, id := range s.CategoryDefaults {
		out.CategoryDefaults[string(c)] = string(id)
	}
	for _, p := range s.Properties {
		kinds := p.Kinds
		if kinds == nil {
			kinds = []string{}
		}
		out.Properties = append(out.Properties, WorkspaceProperty{
			ID: string(p.ID), Key: p.Key, Name: p.Name, Type: string(p.Type),
			Builtin: p.Builtin, Kinds: kinds, Hidden: p.Hidden,
		})
	}
	return out
}
