package workspaces_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func jsonOf(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// Epic のテンプレートを、子 2 つ(1 つは Step 付き)で作る。名前の重複は断る。
func TestCreateTemplate(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	in := workspace.Template{
		Name: " リリース作業 ", Kind: "Epic", Title: "リリース ", Description: "## 手順",
		Values: map[change.ID]json.RawMessage{"priority": jsonOf(st.Priorities[1].ID), "assignee": json.RawMessage("null")},
		Children: []workspace.TemplateChild{
			{Kind: "Story", Title: "QA", Steps: []workspace.TemplateStep{{Title: "確認"}, {Title: " "}}},
			{Kind: "Task", Title: "本番反映"},
			{Kind: "Task", Title: ""},
		},
	}
	got, err := s.CreateTemplate(ctx, store, w.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := settingsOf(t, s, w.ID).Template(got.ID)
	if !ok || saved.Name != "リリース作業" || saved.Title != "リリース " || len(saved.Children) != 2 ||
		len(saved.Children[0].Steps) != 1 || saved.Children[0].Steps[0].Assignee != workspace.StepParent ||
		len(saved.Values) != 1 {
		t.Errorf("saved = %+v", saved)
	}
	_, err = s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "リリース作業", Kind: "Task"})
	rejected(t, err, "同じ名前のテンプレートがあります")
	_, err = s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: " ", Kind: "Task"})
	rejected(t, err, "名前を入力してください")
	if _, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "x", Kind: "Sprint"}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("bad kind: %v", err)
	}
}

// 編集: 送らなかった値は残り、null は外れる。消した Step は消え、種類を変えると既定から外れる。
func TestUpdateTemplate(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	tpl, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{
		Name: "調査", Kind: "Task",
		Values: map[change.ID]json.RawMessage{"priority": jsonOf(st.Priorities[0].ID), "assignee": jsonOf("taro")},
		Steps:  []workspace.TemplateStep{{Title: "再現"}, {Title: "原因"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetTemplateDefault(ctx, store, w.ID, "Task", tpl.ID); err != nil {
		t.Fatal(err)
	}
	keep := tpl.Steps[1]
	err = s.UpdateTemplate(ctx, store, w.ID, tpl.ID, workspace.Template{
		Name: "調査 2", Kind: "Task",
		Values: map[change.ID]json.RawMessage{"assignee": json.RawMessage("null")}, // 優先度は送らない
		Steps:  []workspace.TemplateStep{keep, {Title: "報告", Assignee: workspace.StepNone}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if got.Name != "調査 2" || string(got.Values["priority"]) != string(jsonOf(st.Priorities[0].ID)) || got.Values["assignee"] != nil {
		t.Errorf("values = %+v", got)
	}
	if len(got.Steps) != 2 || got.Steps[0].ID != keep.ID || got.Steps[1].Title != "報告" {
		t.Errorf("steps = %+v", got.Steps)
	}
	if err := s.UpdateTemplate(ctx, store, w.ID, tpl.ID, workspace.Template{Name: "調査 2", Kind: "Bug"}); err != nil {
		t.Fatal(err)
	}
	after := settingsOf(t, s, w.ID)
	if after.TemplateDefaults["Task"] != "" {
		t.Errorf("Task default must be cleared: %v", after.TemplateDefaults)
	}
	if err := s.UpdateTemplate(ctx, store, w.ID, "nope", workspace.Template{Name: "x", Kind: "Task"}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

// 削除で既定からも外れる。既定は同じ種類のテンプレートだけ。
func TestDeleteTemplateAndDefaults(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	tpl, _ := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "不具合", Kind: "Bug"})
	if err := s.SetTemplateDefault(ctx, store, w.ID, "Task", tpl.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("other kind: %v", err)
	}
	if err := s.SetTemplateDefault(ctx, store, w.ID, "Sprint", ""); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("bad kind: %v", err)
	}
	if err := s.SetTemplateDefault(ctx, store, w.ID, "Bug", tpl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTemplate(ctx, store, w.ID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	st := settingsOf(t, s, w.ID)
	if len(st.Templates) != 0 || st.TemplateDefaults["Bug"] != "" {
		t.Errorf("after delete = %+v %v", st.Templates, st.TemplateDefaults)
	}
	if err := s.DeleteTemplate(ctx, store, w.ID, tpl.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("twice: %v", err)
	}
}
