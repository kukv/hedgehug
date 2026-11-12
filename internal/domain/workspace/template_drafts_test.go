package workspace_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const child2 change.ID = "01CHILD00000000000000000C2"

// 作成の行を読み直すと、同じテンプレートになる。並びは送った順に V・W…。
func TestTemplateDraftsRoundTrip(t *testing.T) {
	in := workspace.Template{
		ID: tplA, Name: "リリース作業", Kind: "Epic", Title: "リリース", Description: "## 手順",
		Values: map[change.ID]json.RawMessage{"assignee": raw("taro")},
		Children: []workspace.TemplateChild{
			{ID: child, Kind: "Story", Title: "QA", Steps: []workspace.TemplateStep{{ID: stepC, Title: "確認", Assignee: workspace.StepParent}}},
			{ID: child2, Kind: "Task", Title: "本番反映"},
		},
	}.Ranked()
	if in.Children[0].Rank != "V" || in.Children[1].Rank != "W" || in.Children[0].Steps[0].Rank != "V" {
		t.Errorf("ranks = %+v", in.Children)
	}
	st := workflowSettings(t, func(workspace.Settings) []change.Draft {
		return workspace.TemplateDrafts(workspace.Template{}, in)
	})
	got, ok := st.Template(tplA)
	if !ok || got.Name != in.Name || got.Description != "## 手順" || len(got.Children) != 2 ||
		got.Children[1].Title != "本番反映" || len(got.Children[0].Steps) != 1 || string(got.Values["assignee"]) != `"taro"` {
		t.Errorf("got %+v", got)
	}
}

// 変えた項目だけを行にする。消した子は削除の行、外した値は null。
func TestTemplateDraftsDiff(t *testing.T) {
	old := workspace.Template{
		ID: tplA, Name: "A", Kind: "Epic", Title: "t",
		Values:   map[change.ID]json.RawMessage{"assignee": raw("taro"), "priority": raw("P1")},
		Children: []workspace.TemplateChild{{ID: child, Kind: "Story", Title: "QA", Rank: "V"}, {ID: child2, Kind: "Task", Title: "x", Rank: "W"}},
	}
	next := old
	next.Title = "t2"
	next.Values = map[change.ID]json.RawMessage{"assignee": json.RawMessage(` "taro" `)} // 空白の違いは同じ値
	next.Children = []workspace.TemplateChild{{ID: child, Kind: "Story", Title: "QA", Rank: "V"}}
	want := []change.Draft{
		change.NewSet(tf(tplA, "title"), "t", "t2"),
		change.NewSet(tf(tplA, "values", "priority"), raw("P1"), nil),
		change.NewDelete(tf(tplA, "children", string(child2))),
	}
	if got := workspace.TemplateDrafts(old, next); !sameDrafts(got, want) {
		t.Errorf("drafts = %v\nwant %v", got, want)
	}
}

// 削除は、テンプレートの削除の行と、指していた既定を「なし」にする行。
func TestTemplateDeleteDrafts(t *testing.T) {
	s := workspace.Settings{
		Templates:        []workspace.Template{{ID: tplA, Name: "A", Kind: "Task"}},
		TemplateDefaults: map[string]change.ID{"Task": tplA, "Bug": tplB},
	}
	want := []change.Draft{change.NewDelete(tf(tplA)), change.NewSet("template_default/Task", tplA, nil)}
	if got := workspace.TemplateDeleteDrafts(s, tplA); !sameDrafts(got, want) {
		t.Errorf("drafts = %v", got)
	}
}
