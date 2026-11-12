package workspaces_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestValueEdits(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	statuses := workspace.ParseList("statuses")
	rejected(t, func() error { _, err := s.AddValue(ctx, store, w.ID, statuses, "todo"); return err }(), "同じ名前の値があります")
	review, err := s.AddValue(ctx, store, w.ID, statuses, " Review ")
	if err != nil {
		t.Fatal(err)
	}
	// いまの値は 5 つなので、色は Colors[5](桃)。分類は未着手。
	if review.Name != "Review" || review.Color != workspace.Colors[5] || review.Category != workspace.CategoryTodo {
		t.Errorf("review = %+v", review)
	}
	if err := s.RenameValue(ctx, store, w.ID, statuses, review.ID, "Reviewing"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetValueColor(ctx, store, w.ID, statuses, review.ID, "purple"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatusCategory(ctx, store, w.ID, review.ID, workspace.CategoryDoing); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveValue(ctx, store, w.ID, statuses, review.ID, true); err != nil {
		t.Fatal(err)
	}
	st := settingsOf(t, s, w.ID)
	got := st.Statuses[4]
	if got.ID != review.ID || got.Name != "Reviewing" || got.Color != "purple" || got.Category != workspace.CategoryDoing {
		t.Errorf("statuses = %+v", st.Statuses)
	}
	if err := s.SetValueColor(ctx, store, w.ID, statuses, review.ID, "black"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("black: %v", err)
	}
}

// ない一覧・値は、検査の前に ErrNotFound になる(B2-12)。
func TestValueNotFound(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	statuses := workspace.ParseList("statuses")
	nolist := workspace.ParseList("nope")
	_, addErr := s.AddValue(ctx, store, w.ID, nolist, "x")
	for _, err := range []error{
		addErr,
		s.RenameValue(ctx, store, w.ID, statuses, "nope", "x"),
		s.RenameValue(ctx, store, w.ID, nolist, "nope", "x"),
		s.SetValueColor(ctx, store, w.ID, statuses, "nope", "red"),
		s.SetStatusCategory(ctx, store, w.ID, "nope", workspace.CategoryDoing),
		s.SetStatusCategory(ctx, store, w.ID, settingsOf(t, s, w.ID).Statuses[0].ID, "weird"),
		s.MoveValue(ctx, store, w.ID, statuses, "nope", true),
		s.SetInitialStatus(ctx, store, w.ID, "nope"),
		s.SetInitialPriority(ctx, store, w.ID, "nope"),
		s.SetCategoryDefault(ctx, store, w.ID, workspace.CategoryTodo, "nope"),
		s.DeleteValue(ctx, store, w.ID, statuses, "nope", ""),
		s.DeleteValue(ctx, store, w.ID, nolist, "nope", ""),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("error = %v", err)
		}
	}
}

func TestInitialAndDefaults(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	todo, doing, done, pending := st.Statuses[0].ID, st.Statuses[1].ID, st.Statuses[2].ID, st.Statuses[3].ID
	rejected(t, s.SetInitialStatus(ctx, store, w.ID, done), "完了・中止のステータスは初期ステータスにできません")
	if err := s.SetInitialStatus(ctx, store, w.ID, doing); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCategoryDefault(ctx, store, w.ID, workspace.CategoryTodo, pending); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCategoryDefault(ctx, store, w.ID, workspace.CategoryTodo, done); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("wrong category: %v", err)
	}
	if err := s.SetInitialPriority(ctx, store, w.ID, st.Priorities[0].ID); err != nil {
		t.Fatal(err)
	}
	st = settingsOf(t, s, w.ID)
	if st.InitialStatus != doing || st.CategoryDefaults[workspace.CategoryTodo] != pending || st.InitialPriority != st.Priorities[0].ID {
		t.Errorf("settings = %+v", st)
	}
	// Todo は初期でも既定でもなくなったので消せる。
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("statuses"), todo, ""); err != nil {
		t.Fatal(err)
	}
}

// 値を消すと、使っているアイテムが置き換わる・値が外れる(03「削除」、B2-8)。
func TestDeleteValueReplaces(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	pending, cancel := st.Statuses[3].ID, st.Statuses[4].ID
	high, low := st.Priorities[1].ID, st.Priorities[3].ID
	web, api := st.Systems[0].ID, st.Systems[1].ID
	env, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "環境", Key: "env", Type: workspace.TypeSingle})
	envs := workspace.ParseList(string(env.ID))
	prod, _ := s.AddValue(ctx, store, w.ID, envs, "本番")
	id := putItem(t, s, w.ID,
		change.NewSet("status", nil, pending), change.NewSet("priority", nil, high),
		change.NewAdd("systems", web), change.NewAdd("systems", api),
		change.NewSet(item.FieldKey(env.ID), nil, prod.ID))
	steps := []struct {
		list        string
		value, repl change.ID
	}{
		{"statuses", pending, cancel},
		{"priorities", high, ""}, // 初期優先度(Medium)に置き換える
		{"systems", web, ""},
		{string(env.ID), prod.ID, ""}, // 「なし」
	}
	for _, c := range steps {
		if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList(c.list), c.value, c.repl); err != nil {
			t.Fatalf("%s: %v", c.list, err)
		}
	}
	it := itemOf(t, s, id)
	if it.Status != cancel || it.Priority != st.InitialPriority || !slices.Equal(it.Systems, []change.ID{api}) || it.Choices != nil {
		t.Errorf("item = %+v", it)
	}
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("priorities"), low, low); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("self replacement: %v", err)
	}
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("systems"), api, st.Systems[2].ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("multi replacement: %v", err)
	}
	st = settingsOf(t, s, w.ID)
	if len(st.Statuses) != 4 || len(st.Priorities) != 4 || len(st.Systems) != 5 {
		t.Errorf("settings = %+v", st)
	}
}
