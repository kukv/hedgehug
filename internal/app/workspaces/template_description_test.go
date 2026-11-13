package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// describe は、アイテムの説明を書く(履歴の desc の行も書く)。
func describe(t *testing.T, s workspaces.Service, id item.ID, text string) {
	t.Helper()
	err := s.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(id, text), tx.WriteItems().Append(id, change.NewDesc()))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateTemplateFromItemCopiesTheDescriptions(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	epic := putItemIn(t, s, p, 1)
	qa := putChild(t, s, p, epic, item.Story, 2, "V", "QA")
	putChild(t, s, p, epic, item.Task, 3, "W", "反映")
	putChild(t, s, p, qa, item.Step, 4, "V", "確認")
	describe(t, s, epic, "## 手順\n")
	describe(t, s, qa, "## 確認項目\n")

	tpl, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース作業", Title: "リリース", Structure: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if got.Description != "## 手順\n" || len(got.Children) != 2 {
		t.Fatalf("template = %+v", got)
	}
	// 子は説明を写す。説明のない子は空のまま。Step は説明を持たない。
	if got.Children[0].Title != "QA" || got.Children[0].Description != "## 確認項目\n" || got.Children[1].Description != "" {
		t.Errorf("children = %+v", got.Children)
	}

	// 子と Step を含めないなら、アイテムの説明だけを写す。
	plain, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース 2", Title: "リリース", Structure: false})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := settingsOf(t, s, w.ID).Template(plain.ID); got.Description != "## 手順\n" || len(got.Children) != 0 {
		t.Errorf("plain template = %+v", got)
	}
}

func TestCreateTemplateFromItemWithoutADescription(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	epic := putItemIn(t, s, p, 1)
	tpl, err := s.CreateTemplateFromItem(t.Context(), store, epic, workspaces.FromItem{Name: "作業", Title: "作業"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := settingsOf(t, s, w.ID).Template(tpl.ID); got.Description != "" {
		t.Errorf("description = %q", got.Description)
	}
}
