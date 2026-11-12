package workspaces_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// putChild は、親 parent の下に種類 kind のアイテムを作る(番号 number、並び rank)。
func putChild(t *testing.T, s workspaces.Service, p project.Project, parent item.ID, kind item.Kind, number int, rank, title string, drafts ...change.Draft) item.ID {
	t.Helper()
	it := item.Item{ID: s.IDs.New(), Place: item.Place{Workspace: p.Place.Workspace, Project: p.ID, Parent: &parent, Number: number}, Kind: kind, Rank: rank, Title: title}
	err := s.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(it.ID, append([]change.Draft{item.Create(it)}, drafts...)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func TestCreateTemplateFromItem(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	epic := putItemIn(t, s, p, 1,
		change.NewSet("assignee", nil, "taro"), change.NewSet("priority", nil, st.Priorities[1].ID),
		change.NewSet("status", nil, st.Statuses[1].ID), change.NewSet("points", nil, 3))
	qa := putChild(t, s, p, epic, item.Story, 2, "V", "QA", change.NewSet("assignee", nil, "hanako"))
	putChild(t, s, p, qa, item.Step, 3, "W", "記録")
	putChild(t, s, p, qa, item.Step, 4, "V", "確認", change.NewSet("assignee", nil, "hanako"))
	putChild(t, s, p, qa, item.Step, 5, "X", "報告", change.NewSet("assignee", nil, "jiro"))

	tpl, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース作業", Title: "リリース", Structure: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if got.Kind != "Epic" || got.Title != "リリース" || len(got.Children) != 1 {
		t.Fatalf("template = %+v", got)
	}
	// ステータスとポイントは写さない。Epic は担当者を使う種類なら担当者を写す(既定の設定は全種類で使う)。
	if got.Values["status"] != nil || got.Values["points"] != nil || string(got.Values["assignee"]) != `"taro"` {
		t.Errorf("values = %v", got.Values)
	}
	c := got.Children[0]
	var steps [][2]string
	for _, st := range c.Steps {
		steps = append(steps, [2]string{st.Title, st.Assignee})
	}
	want := [][2]string{{"確認", workspace.StepParent}, {"記録", workspace.StepNone}, {"報告", "jiro"}}
	if c.Kind != "Story" || c.Title != "QA" || len(steps) != 3 || steps[0] != want[0] || steps[1] != want[1] || steps[2] != want[2] {
		t.Errorf("child = %+v steps = %v", c, steps)
	}

	// 含めないなら、構成は写さない。名前の重複は断る。
	plain, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース 2", Title: "リリース", Structure: false})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := settingsOf(t, s, w.ID).Template(plain.ID); len(got.Children) != 0 {
		t.Errorf("children copied: %+v", got.Children)
	}
	_, err = s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース作業", Title: "x"})
	rejected(t, err, "同じ名前のテンプレートがあります")
}
