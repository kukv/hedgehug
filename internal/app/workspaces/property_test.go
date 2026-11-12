package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// settingsOf は、保存されている設定を返す。
func settingsOf(t *testing.T, s workspaces.Service, ws workspace.ID) workspace.Settings {
	t.Helper()
	var st workspace.Settings
	if err := s.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		st, err = r.Workspaces().Settings(ws)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return st
}

// putItem は、ws にプロジェクトとアイテムを 1 つ作り、drafts を足す。アイテムの ID を返す。
func putItem(t *testing.T, s workspaces.Service, ws workspace.ID, drafts ...change.Draft) item.ID {
	t.Helper()
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: ws, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	return putItemIn(t, s, p, 1, drafts...)
}

// putItemIn は、プロジェクト p に番号 number のアイテムを作り、drafts を足す。番号 1 のときだけプロジェクトも作る。
func putItemIn(t *testing.T, s workspaces.Service, p project.Project, number int, drafts ...change.Draft) item.ID {
	t.Helper()
	ws := p.Place.Workspace
	it := item.Item{ID: s.IDs.New(), Place: item.Place{Workspace: ws, Project: p.ID, Number: number}, Kind: item.Epic, Rank: "V", Title: "x"}
	err := s.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		var perr error
		if number == 1 {
			perr = tx.WriteProjects().Append(p.ID, project.Create(p))
		}
		return errors.Join(
			perr,
			tx.WriteItems().Append(it.ID, append([]change.Draft{item.Create(it)}, drafts...)...),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func itemOf(t *testing.T, s workspaces.Service, id item.ID) item.Item {
	t.Helper()
	var it item.Item
	if err := s.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		it, err = r.Items().Get(id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return it
}

func TestAddProperty(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	p, err := s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: " 顧客 ", Key: "customer", Type: workspace.TypeSingle})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := settingsOf(t, s, w.ID).Property(p.ID)
	if !ok || got.Name != "顧客" || got.Builtin || len(got.Kinds) != 5 || got.Rank <= settingsOf(t, s, w.ID).Properties[7].Rank {
		t.Errorf("property = %+v", got)
	}
	_, err = s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: "顧客", Key: "c2", Type: workspace.TypeText})
	rejected(t, err, "同じ名前のプロパティがあります")
	_, err = s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: "x", Key: "x", Type: workspace.TypeDuration})
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("duration: %v", err)
	}
}

func TestEditProperty(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	ctx := t.Context()
	rejected(t, s.RenameProperty(ctx, store, w.ID, "assignee", "優先度"), "同じ名前のプロパティがあります")
	if err := s.RenameProperty(ctx, store, w.ID, "assignee", "担当"); err != nil {
		t.Fatal(err)
	}
	rejected(t, s.SetPropertyKinds(ctx, store, w.ID, "assignee", nil), "少なくとも 1 つの種類で使ってください")
	if err := s.SetPropertyKinds(ctx, store, w.ID, "assignee", []string{"Task", "Bug"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPropertyHidden(ctx, store, w.ID, "points", true); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveProperty(ctx, store, w.ID, "assignee", true); err != nil { // ステータスの上へ
		t.Fatal(err)
	}
	st := settingsOf(t, s, w.ID)
	a, _ := st.Property("assignee")
	pts, _ := st.Property("points")
	if a.Name != "担当" || len(a.Kinds) != 2 || !pts.Hidden || st.Properties[0].ID != "assignee" {
		t.Errorf("settings = %+v", st.Properties)
	}
	for _, err := range []error{
		s.SetPropertyKinds(ctx, store, w.ID, "status", []string{"Task"}),
		s.SetPropertyHidden(ctx, store, w.ID, "status", true),
		s.DeleteProperty(ctx, store, w.ID, "assignee"),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("fixed: %v", err)
		}
	}
	// 先頭で上へは、何も保存しない。
	_, before := stored(t, s, w.ID)
	if err := s.MoveProperty(ctx, store, w.ID, "assignee", true); err != nil {
		t.Fatal(err)
	}
	if _, after := stored(t, s, w.ID); len(after) != len(before) {
		t.Error("moved at the top")
	}
}

// ない ID は、どの操作も ErrNotFound になる(B2-12)。
func TestPropertyNotFound(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	ctx := t.Context()
	for _, err := range []error{
		s.RenameProperty(ctx, store, w.ID, "nope", "x"),
		s.SetPropertyKinds(ctx, store, w.ID, "nope", []string{"Task"}),
		s.SetPropertyHidden(ctx, store, w.ID, "nope", true),
		s.MoveProperty(ctx, store, w.ID, "nope", true),
		s.DeleteProperty(ctx, store, w.ID, "nope"),
		s.RenameProperty(ctx, store, "nows", "assignee", "x"),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("error = %v", err)
		}
	}
}

// 追加プロパティを消すと、アイテムの値も消える(03「削除」)。
func TestDeleteProperty(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tags, _ := s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: "タグ", Key: "tags", Type: workspace.TypeMulti})
	one, _ := s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: "顧客", Key: "customer", Type: workspace.TypeSingle})
	a, _ := s.AddValue(t.Context(), store, w.ID, workspace.ParseList(string(tags.ID)), "A")
	b, _ := s.AddValue(t.Context(), store, w.ID, workspace.ParseList(string(tags.ID)), "B")
	c, _ := s.AddValue(t.Context(), store, w.ID, workspace.ParseList(string(one.ID)), "C")
	id := putItem(t, s, w.ID,
		change.NewAdd(item.FieldKey(tags.ID), a.ID),
		change.NewAdd(item.FieldKey(tags.ID), b.ID),
		change.NewSet(item.FieldKey(one.ID), nil, c.ID))
	if it := itemOf(t, s, id); len(it.Choices[tags.ID]) != 2 || len(it.Choices[one.ID]) != 1 {
		t.Fatalf("precondition: choices = %v", it.Choices)
	}
	for _, p := range []change.ID{tags.ID, one.ID} {
		if err := s.DeleteProperty(t.Context(), store, w.ID, p); err != nil {
			t.Fatal(err)
		}
		if _, ok := settingsOf(t, s, w.ID).Property(p); ok {
			t.Error("property remains")
		}
	}
	if it := itemOf(t, s, id); len(it.Choices[tags.ID]) != 0 || len(it.Choices[one.ID]) != 0 {
		t.Errorf("choices = %v", it.Choices)
	}
}
