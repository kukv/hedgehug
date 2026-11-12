package workspace_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

const envProp change.ID = "01P0000000000000000000000E"

// 名前の前後の空白を除き(タイトルの書き出しはそのまま)、空の Step を捨て、種類で使わない値・持たない値・持てない構成を捨てる。
func TestNormalizeTemplate(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	s.Properties = append(s.Properties, workspace.Property{ID: envProp, Key: "env", Name: "環境", Type: workspace.TypeText, Kinds: []string{"Story"}})
	in := workspace.Template{
		Name: " 調査 ", Kind: "Task", Title: " 調査: ",
		Values: map[change.ID]json.RawMessage{
			"priority": raw(s.Priorities[0].ID), envProp: raw("本番"), "status": raw(s.Statuses[0].ID), "systems": raw([]string{}),
		},
		Steps:    []workspace.TemplateStep{{Title: " 再現 "}, {Title: "  "}},
		Children: []workspace.TemplateChild{{Kind: "Story", Title: "捨てる"}},
	}
	got := workspace.NormalizeTemplate(s, in)
	if got.Name != "調査" || got.Title != " 調査: " {
		t.Errorf("name/title = %q %q", got.Name, got.Title)
	}
	if len(got.Values) != 1 || got.Values["priority"] == nil {
		t.Errorf("values = %v", got.Values)
	}
	if len(got.Steps) != 1 || got.Steps[0].Title != "再現" || got.Steps[0].Assignee != workspace.StepParent || got.Children != nil {
		t.Errorf("structure = %+v %+v", got.Steps, got.Children)
	}
}

func TestCheckTemplate(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	s.Templates = []workspace.Template{{ID: tplA, Name: "調査", Kind: "Task"}}
	cases := []struct {
		name, want string
		id         change.ID
	}{
		{"", "名前を入力してください", ""},
		{"調査", "同じ名前のテンプレートがあります", ""},
		{"調査", "", tplA}, // 自分の名前のまま
		{"調査 2", "", ""},
		{"ちょうさ", "", ""},
	}
	for _, c := range cases {
		if got := refusedWith(workspace.CheckTemplate(s, workspace.Template{ID: c.id, Name: c.name, Kind: "Task"})); got != c.want {
			t.Errorf("%q: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDroppedByKind(t *testing.T) {
	epic := workspace.Template{Kind: "Epic", Children: []workspace.TemplateChild{
		{Title: "QA", Steps: []workspace.TemplateStep{{Title: "a"}, {Title: "b"}}}, {Title: "本番"},
	}}
	task := workspace.Template{Kind: "Task", Steps: []workspace.TemplateStep{{Title: "a"}}}
	for _, c := range []struct {
		t        workspace.Template
		kind     string
		children int
		steps    int
	}{{epic, "Task", 2, 2}, {epic, "Epic", 0, 0}, {task, "Bug", 0, 0}, {task, "Step", 0, 1}, {task, "Epic", 0, 1}} {
		if ch, st := workspace.DroppedByKind(c.t, c.kind); ch != c.children || st != c.steps {
			t.Errorf("%s→%s = %d %d", c.t.Kind, c.kind, ch, st)
		}
	}
}

func TestValidTemplate(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	ok := workspace.Template{Name: "a", Kind: "Epic", Values: map[change.ID]json.RawMessage{
		"priority": raw(s.Priorities[0].ID), "systems": raw([]change.ID{s.Systems[0].ID}), "assignee": raw("taro"),
	}, Children: []workspace.TemplateChild{{Kind: "Story", Title: "QA"}}}
	if !workspace.ValidTemplate(s, ok) {
		t.Error("valid template refused")
	}
	for name, bad := range map[string]workspace.Template{
		"kind":       {Name: "a", Kind: "Sprint"},
		"child kind": {Name: "a", Kind: "Epic", Children: []workspace.TemplateChild{{Kind: "Epic", Title: "x"}}},
		"priority":   {Name: "a", Kind: "Task", Values: map[change.ID]json.RawMessage{"priority": raw("nope")}},
		"systems":    {Name: "a", Kind: "Task", Values: map[change.ID]json.RawMessage{"systems": raw([]string{"nope"})}},
		"property":   {Name: "a", Kind: "Task", Values: map[change.ID]json.RawMessage{"nope": raw("x")}},
		"status":     {Name: "a", Kind: "Task", Values: map[change.ID]json.RawMessage{"status": raw(s.Statuses[0].ID)}},
	} {
		if workspace.ValidTemplate(s, bad) {
			t.Errorf("%s: invalid template accepted", name)
		}
	}
}
