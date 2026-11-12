package workspaces_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ステータス Review を消すと、遷移の制約から外れ、置き換えのアイテムは制約を通さずに変わる(B2-10)。
func TestDeleteStatusFollowsWorkflow(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	statuses := workspace.ParseList("statuses")
	review, _ := s.AddValue(ctx, store, w.ID, statuses, "Review")
	st := settingsOf(t, s, w.ID)
	todo, done := st.Statuses[0].ID, st.Statuses[2].ID
	for _, p := range [][2]change.ID{{todo, review.ID}, {review.ID, done}} {
		if err := s.SetTransition(ctx, store, w.ID, p[0], p[1], true); err != nil {
			t.Fatal(err)
		}
	}
	it := putItem(t, s, w.ID, change.NewSet("status", nil, review.ID))
	if err := s.DeleteValue(ctx, store, w.ID, statuses, review.ID, done); err != nil {
		t.Fatal(err)
	}
	after := settingsOf(t, s, w.ID)
	if _, r := after.Allowed(todo); r || len(after.Workflow[review.ID]) != 0 {
		t.Errorf("workflow = %v", after.Workflow)
	}
	if got := itemOf(t, s, it); got.Status != done {
		t.Errorf("status = %s", got.Status)
	}
}

// 優先度・追加プロパティを消すと、テンプレートの値が置き換わる・外れる。
func TestDeleteValueAndPropertyFollowTemplates(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	env, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "環境", Key: "env", Type: workspace.TypeText})
	high := st.Priorities[1].ID
	tpl, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "調査", Kind: "Task", Values: map[change.ID]json.RawMessage{
		"priority": jsonOf(high), env.ID: jsonOf("本番"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("priorities"), high, st.Priorities[3].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProperty(ctx, store, w.ID, env.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if string(got.Values["priority"]) != string(jsonOf(st.Priorities[3].ID)) || got.Values[env.ID] != nil {
		t.Errorf("values = %v", got.Values)
	}
}
