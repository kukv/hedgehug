package workspace_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workflowSettings は、既定の設定の作成の行に drafts を足して読み直した設定。
func workflowSettings(t *testing.T, drafts func(s workspace.Settings) []change.Draft) workspace.Settings {
	t.Helper()
	w := workspace.Workspace{ID: "01WS0000000000000000000000", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(seqIDs())
	create, err := workspace.CreateWithSettings(w, s)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := workspace.SettingsFromLog(rows(append([]change.Draft{create}, drafts(s)...)...))
	if !ok {
		t.Fatal("no create")
	}
	return got
}

// refusedWith は、断りの文言。断りでなければ空。
func refusedWith(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

// sameDrafts は、a と b が同じ行の集合か(並びは問わない)。
func sameDrafts(a, b []change.Draft) bool {
	if len(a) != len(b) {
		return false
	}
	used := make([]bool, len(b))
	for _, x := range a {
		found := false
		for i, y := range b {
			if !used[i] && reflect.DeepEqual(x, y) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Todo から InProgress・Done を許し、Done を外して Cancel を足す。
func TestWorkflowFromLog(t *testing.T) {
	st := workflowSettings(t, func(s workspace.Settings) []change.Draft {
		todo, doing, done, cancel := s.Statuses[0].ID, s.Statuses[1].ID, s.Statuses[2].ID, s.Statuses[4].ID
		f := workspace.WorkflowField(todo)
		return []change.Draft{
			change.NewAdd(f, done), change.NewAdd(f, doing),
			change.NewRemove(f, done), change.NewAdd(f, cancel),
		}
	})
	got, restricted := st.Allowed(st.Statuses[0].ID)
	want := []change.ID{st.Statuses[1].ID, st.Statuses[4].ID} // ステータスの並び(InProgress・Cancel)
	if !restricted || !reflect.DeepEqual(got, want) {
		t.Errorf("Allowed(Todo) = %v %v, want %v", got, restricted, want)
	}
	if _, r := st.Allowed(st.Statuses[1].ID); r {
		t.Error("InProgress has no row and must be unrestricted")
	}
}

func TestCheckTransition(t *testing.T) {
	st := workflowSettings(t, func(s workspace.Settings) []change.Draft {
		f := workspace.WorkflowField(s.Statuses[0].ID)
		return []change.Draft{change.NewAdd(f, s.Statuses[1].ID), change.NewAdd(f, s.Statuses[3].ID)}
	})
	todo, doing, done, pending := st.Statuses[0].ID, st.Statuses[1].ID, st.Statuses[2].ID, st.Statuses[3].ID
	for _, ok := range [][2]change.ID{{todo, doing}, {todo, pending}, {todo, todo}, {done, todo}, {"", done}} {
		if err := workspace.CheckTransition(st, ok[0], ok[1]); err != nil {
			t.Errorf("%v: %v", ok, err)
		}
	}
	want := "ワークフローの制約: Todo から Done には変更できません(InProgress・Pending のみ)"
	if got := refusedWith(workspace.CheckTransition(st, todo, done)); got != want {
		t.Errorf("refusal = %q, want %q", got, want)
	}
}

// 消す行は、その行の許可先と、ほかの行のそのステータス。削除したステータスは許可先に数えない。
func TestWorkflowDeleteDrafts(t *testing.T) {
	var todo, doing, done change.ID
	st := workflowSettings(t, func(s workspace.Settings) []change.Draft {
		todo, doing, done = s.Statuses[0].ID, s.Statuses[1].ID, s.Statuses[2].ID
		return []change.Draft{
			change.NewAdd(workspace.WorkflowField(todo), doing),
			change.NewAdd(workspace.WorkflowField(doing), done),
			change.NewAdd(workspace.WorkflowField(doing), todo),
		}
	})
	want := []change.Draft{
		change.NewRemove(workspace.WorkflowField(doing), done),
		change.NewRemove(workspace.WorkflowField(doing), todo),
		change.NewRemove(workspace.WorkflowField(todo), doing),
	}
	if got := workspace.WorkflowDeleteDrafts(st, doing); !sameDrafts(got, want) {
		t.Errorf("drafts = %v, want %v", got, want)
	}
	after := workflowSettings(t, func(workspace.Settings) []change.Draft {
		return []change.Draft{
			change.NewAdd(workspace.WorkflowField(todo), doing),
			change.NewDelete("statuses/" + string(doing)),
		}
	})
	if _, r := after.Allowed(todo); r {
		t.Error("a row whose only target was deleted must be unrestricted")
	}
}
