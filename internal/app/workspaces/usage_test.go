package workspaces_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestUsage(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	env, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "環境", Key: "env", Type: workspace.TypeSingle})
	prod, _ := s.AddValue(ctx, store, w.ID, workspace.ParseList(string(env.ID)), "本番")
	putItem(t, s, w.ID, change.NewSet("status", nil, st.Statuses[0].ID), change.NewAdd("systems", st.Systems[0].ID),
		change.NewSet(item.FieldKey(env.ID), nil, prod.ID))
	u, err := s.Usage(ctx, store, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Items != 1 || !slices.Equal(u.FirstNumbers, []int{1}) || u.Values[st.Statuses[0].ID] != 1 ||
		u.Values[st.Systems[0].ID] != 1 || u.Values[prod.ID] != 1 || u.Properties[env.ID] != 1 || u.Values[st.Statuses[1].ID] != 0 {
		t.Errorf("usage = %+v", u)
	}
	if _, err := s.Usage(ctx, store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

// 番号は小さい方から 3 つだけ返し、件数は全部数える。アイテムのないワークスペースでもマップは nil にしない。
func TestUsageFirstNumbers(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	empty, err := s.Usage(ctx, store, w.ID)
	if err != nil || empty.Items != 0 || empty.Values == nil || empty.Properties == nil || len(empty.FirstNumbers) != 0 {
		t.Fatalf("empty = %+v, %v", empty, err)
	}
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	for _, n := range []int{1, 2, 3, 4} {
		putItemIn(t, s, p, n)
	}
	u, err := s.Usage(ctx, store, w.ID)
	if err != nil || u.Items != 4 || !slices.Equal(u.FirstNumbers, []int{1, 2, 3}) {
		t.Errorf("usage = %+v, %v", u, err)
	}
}
