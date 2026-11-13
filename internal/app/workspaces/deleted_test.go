package workspaces_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 使っているアイテムの件数と接頭辞の例は、削除済みのアイテムと、削除済みのプロジェクトのアイテムを数えない(B2-11・A4b-5)。
func TestUsageSkipsDeletedItemsAndProjects(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	todo := change.NewSet("status", nil, st.Statuses[0].ID)
	pa := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "a", Status: project.Todo}
	pb := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 2}, Rank: "W", Title: "b", Status: project.Todo}
	first := putItemIn(t, s, pa, 1, todo)
	putItemIn(t, s, pa, 2, todo)
	putItemIn(t, s, pa, 3)
	putItemIn(t, s, pb, 1, todo) // pb のアイテム(番号は pa と重なるが、数えるだけなので構わない)
	usage := func() workspaces.Usage {
		u, err := s.Usage(ctx, store, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	if u := usage(); u.Items != 4 || u.Values[st.Statuses[0].ID] != 3 {
		t.Fatalf("before deleting: %+v", u)
	}
	if err := apptest.DeleteItems(ctx, s.Stores, store, first); err != nil {
		t.Fatal(err)
	}
	if u := usage(); u.Items != 3 || !slices.Equal(u.FirstNumbers, []int{1, 2, 3}) || u.Values[st.Statuses[0].ID] != 2 {
		t.Errorf("after deleting an item: %+v", u)
	}
	if err := apptest.DeleteProject(ctx, s.Stores, store, pb.ID); err != nil {
		t.Fatal(err)
	}
	if u := usage(); u.Items != 2 || !slices.Equal(u.FirstNumbers, []int{2, 3}) || u.Values[st.Statuses[0].ID] != 1 {
		t.Errorf("after deleting a project: %+v", u)
	}
}

// 値を消す置き換えは、削除済みのアイテムも含める(03「削除」。復元したときに、消えた値を指さないため。
// 件数の表示には入らない)。
func TestDeleteValueAlsoReplacesDeletedItems(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	pending, cancel := st.Statuses[3].ID, st.Statuses[4].ID
	id := putItem(t, s, w.ID, change.NewSet("status", nil, pending))
	if err := apptest.DeleteItems(ctx, s.Stores, store, id); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.Usage(ctx, store, w.ID); u.Values[pending] != 0 {
		t.Errorf("a deleted item is not counted: %+v", u)
	}
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("statuses"), pending, cancel); err != nil {
		t.Fatal(err)
	}
	if got := itemOf(t, s, id); got.Status != cancel || got.Deleted == nil {
		t.Errorf("the deleted item = status %q, deleted %v; want it replaced and still deleted", got.Status, got.Deleted)
	}
}

// プロパティを消すと、削除済みのアイテムからも値を外す(03「削除」。値の置き換えと同じ理由。A4b-5)。
func TestDeletePropertyAlsoClearsDeletedItems(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	memo, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "メモ", Key: "memo", Type: workspace.TypeText})
	id := putItem(t, s, w.ID, change.NewSet(item.FieldKey(memo.ID), nil, "下書き"))
	if err := apptest.DeleteItems(ctx, s.Stores, store, id); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProperty(ctx, store, w.ID, memo.ID); err != nil {
		t.Fatal(err)
	}
	if got := itemOf(t, s, id); len(got.Fields) != 0 || got.Deleted == nil {
		t.Errorf("the deleted item = fields %v, deleted %v; want the value cleared and still deleted", got.Fields, got.Deleted)
	}
}

// 「テンプレートとして保存…」は、削除済みのアイテムからは作れず、削除済みの子は写さない(A4b-5)。
func TestCreateTemplateFromItemSkipsDeleted(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	epic := putItemIn(t, s, p, 1)
	putChild(t, s, p, epic, item.Story, 2, "V", "残す")
	gone := putChild(t, s, p, epic, item.Story, 3, "W", "消す")
	if err := apptest.DeleteItems(ctx, s.Stores, store, gone); err != nil {
		t.Fatal(err)
	}
	tpl, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "構成", Title: "x", Structure: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := settingsOf(t, s, w.ID).Template(tpl.ID); len(got.Children) != 1 || got.Children[0].Title != "残す" {
		t.Errorf("children = %+v", got.Children)
	}
	_, err = s.CreateTemplateFromItem(ctx, store, gone, workspaces.FromItem{Name: "構成 2", Title: "x"})
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("from a deleted item: %v", err)
	}
}
