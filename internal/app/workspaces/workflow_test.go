package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Todo の行に InProgress をチェックすると、Todo から Done へは変えられなくなる。外すと戻る。
func TestSetTransition(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	todo, doing, done := st.Statuses[0].ID, st.Statuses[1].ID, st.Statuses[2].ID
	if err := s.SetTransition(ctx, store, w.ID, todo, doing, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTransition(ctx, store, w.ID, todo, doing, true); err != nil { // 同じ: 何もしない
		t.Fatal(err)
	}
	st = settingsOf(t, s, w.ID)
	rejected(t, workspace.CheckTransition(st, todo, done), "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)")
	if err := s.SetTransition(ctx, store, w.ID, todo, doing, false); err != nil {
		t.Fatal(err)
	}
	if _, r := settingsOf(t, s, w.ID).Allowed(todo); r {
		t.Error("the row must be unrestricted again")
	}
	for _, pair := range [][2]change.ID{{todo, todo}, {todo, "nope"}, {"nope", todo}} {
		if err := s.SetTransition(ctx, store, w.ID, pair[0], pair[1], true); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%v: %v", pair, err)
		}
	}
}
