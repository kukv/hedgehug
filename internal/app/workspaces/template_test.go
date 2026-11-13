package workspaces_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
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

// release は、子「QA」(Step「確認」付き)・子「本番反映」・子「告知」の Epic のテンプレート「リリース」を作り、Epic の既定にする。
func release(t *testing.T, s workspaces.Service, ws workspace.ID) workspace.Template {
	t.Helper()
	tpl, err := s.CreateTemplate(t.Context(), store, ws, workspace.Template{
		Name: "リリース", Kind: "Epic",
		Children: []workspace.TemplateChild{
			{Kind: "Task", Title: "QA", Steps: []workspace.TemplateStep{{Title: "確認"}}},
			{Kind: "Task", Title: "本番反映"},
			{Kind: "Task", Title: "告知"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetTemplateDefault(t.Context(), store, ws, "Epic", tpl.ID); err != nil {
		t.Fatal(err)
	}
	return tpl
}

// deleteChild は、別のタブの保存に当たる、テンプレート tpl から子 i を除いた保存。
func deleteChild(t *testing.T, s workspaces.Service, ws workspace.ID, tpl workspace.Template, i int) {
	t.Helper()
	keepChildren(t, s, ws, tpl, slices.Delete(slices.Clone(tpl.Children), i, i+1)...)
}

// keepChildren は、別のタブの保存に当たる、テンプレート tpl の子を cs だけにする保存。
func keepChildren(t *testing.T, s workspaces.Service, ws workspace.ID, tpl workspace.Template, cs ...workspace.TemplateChild) {
	t.Helper()
	in := tpl
	in.Children = cs
	if err := s.UpdateTemplate(t.Context(), store, ws, tpl.ID, in); err != nil {
		t.Fatal(err)
	}
}

// 別のタブで子を 1 つずつ削除して並びが詰まったあと、削除の前のまま全部を送っても、子は削除済みのまま。
func TestUpdateTemplateWithChildrenDeletedOneByOne(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tpl := release(t, s, w.ID)
	deleteChild(t, s, w.ID, tpl, 1)                // 「告知」の rank が詰まる
	keepChildren(t, s, w.ID, tpl, tpl.Children[0]) // 「告知」も削除
	ctx := port.WithRevived(t.Context())
	in := tpl
	in.Name = "リリース 2"
	if err := s.UpdateTemplate(ctx, store, w.ID, tpl.ID, in); err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if got.Name != "リリース 2" || len(got.Children) != 1 {
		t.Errorf("template = %+v", got)
	}
	if n := restores(t, s, w.ID); n != 0 || len(port.Revived(ctx)) != 0 {
		t.Errorf("restore rows = %d, Revived = %v; want none", n, port.Revived(ctx))
	}
}

// 別のタブの 1 回の保存で子 2 つを削除したあと、1 つだけを変えて保存すると、変えた子だけが戻る。
func TestUpdateTemplateWithOneOfDeletedChildren(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tpl := release(t, s, w.ID)
	keepChildren(t, s, w.ID, tpl, tpl.Children[0])
	ctx := port.WithRevived(t.Context())
	in := tpl
	in.Children = slices.Clone(tpl.Children)
	in.Children[1].Title = "本番リリース"
	if err := s.UpdateTemplate(ctx, store, w.ID, tpl.ID, in); err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if len(got.Children) != 2 || got.Children[1].Title != "本番リリース" {
		t.Errorf("children = %+v", got.Children)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "本番反映"}}; !slices.Equal(port.Revived(ctx), want) {
		t.Errorf("Revived = %v, want %v", port.Revived(ctx), want)
	}
}

// 別のタブで削除されたテンプレートを保存すると、削除を取り消して変更を残す。子と Step、既定も戻る。
func TestUpdateDeletedTemplateRevives(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tpl := release(t, s, w.ID)
	if err := s.DeleteTemplate(t.Context(), store, w.ID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	ctx := port.WithRevived(t.Context())
	in := tpl
	in.Name = "リリース 2"
	if err := s.UpdateTemplate(ctx, store, w.ID, tpl.ID, in); err != nil {
		t.Fatal(err)
	}
	st := settingsOf(t, s, w.ID)
	got, ok := st.Template(tpl.ID)
	if !ok || got.Name != "リリース 2" || len(got.Children) != 3 || len(got.Children[0].Steps) != 1 {
		t.Errorf("template = %+v, %v", got, ok)
	}
	if st.TemplateDefaults["Epic"] != tpl.ID {
		t.Errorf("defaults = %v", st.TemplateDefaults)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "リリース"}}; !slices.Equal(port.Revived(ctx), want) {
		t.Errorf("Revived = %v, want %v", port.Revived(ctx), want)
	}
}

// 別のタブで削除された子の題名を変えて保存すると、子が戻り、題名が変わる。
func TestUpdateTemplateWithEditedDeletedChild(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tpl := release(t, s, w.ID)
	deleteChild(t, s, w.ID, tpl, 1)
	ctx := port.WithRevived(t.Context())
	in := tpl
	in.Children = slices.Clone(tpl.Children)
	in.Children[1].Title = "本番リリース"
	if err := s.UpdateTemplate(ctx, store, w.ID, tpl.ID, in); err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if len(got.Children) != 3 || got.Children[1].ID != tpl.Children[1].ID || got.Children[1].Title != "本番リリース" {
		t.Errorf("children = %+v", got.Children)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "本番反映"}}; !slices.Equal(port.Revived(ctx), want) {
		t.Errorf("Revived = %v, want %v", port.Revived(ctx), want)
	}
}

// 別のタブで削除された子を削除の前のまま含め、名前だけを変えて保存すると、子は削除済みのままで、
// 子の項目名の行を書かない(決定 R5、Review Focus 3)。
func TestUpdateTemplateWithUnchangedDeletedChild(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tpl := release(t, s, w.ID)
	deleteChild(t, s, w.ID, tpl, 1)
	before := len(logOf(t, s, w.ID))
	ctx := port.WithRevived(t.Context())
	in := tpl
	in.Name = "リリース 2"
	if err := s.UpdateTemplate(ctx, store, w.ID, tpl.ID, in); err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if got.Name != "リリース 2" || len(got.Children) != 2 || got.Children[0].ID != tpl.Children[0].ID {
		t.Errorf("template = %+v", got)
	}
	prefix := workspace.TemplateField(tpl.ID, "children", string(tpl.Children[1].ID)) + "/"
	for _, r := range logOf(t, s, w.ID)[before:] {
		if strings.HasPrefix(r.F, prefix) || r.T == change.Restore {
			t.Errorf("unexpected row %+v", r)
		}
	}
	if got := port.Revived(ctx); len(got) != 0 {
		t.Errorf("Revived = %v, want none", got)
	}
}

// 削除済みのテンプレートを、削除の前と同じ内容で保存すると、取り消さず ErrNotFound(決定 R3)。
func TestUpdateDeletedTemplateUnchangedIsNotFound(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tpl := release(t, s, w.ID)
	if err := s.DeleteTemplate(t.Context(), store, w.ID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTemplate(port.WithRevived(t.Context()), store, w.ID, tpl.ID, tpl); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want port.ErrNotFound", err)
	}
	if n := restores(t, s, w.ID); n != 0 {
		t.Errorf("restore rows = %d, want 0", n)
	}
}

// 削除したあと同じ名前のテンプレートを作ったら、古い方を名前を変えずに保存すると断る(決定 R4)。
func TestUpdateDeletedTemplateNameTaken(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tpl := release(t, s, w.ID)
	if err := s.DeleteTemplate(t.Context(), store, w.ID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTemplate(t.Context(), store, w.ID, workspace.Template{Name: "リリース", Kind: "Task"}); err != nil {
		t.Fatal(err)
	}
	before := len(logOf(t, s, w.ID))
	in := tpl
	in.Title = "リリース作業"
	rejected(t, s.UpdateTemplate(port.WithRevived(t.Context()), store, w.ID, tpl.ID, in), "同じ名前のテンプレートがあります")
	if after := len(logOf(t, s, w.ID)); after != before {
		t.Errorf("log rows %d -> %d; nothing must be saved", before, after)
	}
}
