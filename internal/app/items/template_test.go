package items_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// saveTemplate は、サンプルのワークスペースにテンプレートを書く(B3 の行の形。app の別のパッケージは使えない)。
func saveTemplate(t *testing.T, svc items.Service, rec *recorder, tp workspace.Template) workspace.Template {
	t.Helper()
	tp.ID = svc.IDs.New()
	for i := range tp.Children {
		tp.Children[i].ID = svc.IDs.New()
		for j := range tp.Children[i].Steps {
			tp.Children[i].Steps[j].ID = svc.IDs.New()
		}
	}
	for j := range tp.Steps {
		tp.Steps[j].ID = svc.IDs.New()
	}
	tp = tp.Ranked()
	err := rec.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("hanako"), Summary: "テンプレートを作成"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, workspace.TemplateDrafts(workspace.Template{}, tp)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return tp
}

// byNumber は、プロジェクトのアイテムを番号で引く。
func byNumber(t *testing.T, svc items.Service) map[int]item.Item {
	t.Helper()
	list, err := svc.List(t.Context(), store, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	out := map[int]item.Item{}
	for _, it := range list {
		out[it.Place.Number] = it
	}
	return out
}

// checkInitials は、テンプレートで作ったアイテムが初期のステータス・優先度と、作った人のウォッチャーを持つことを確かめる。
func checkInitials(t *testing.T, its ...item.Item) {
	t.Helper()
	for _, it := range its {
		if it.Status != apptest.SampleStatusTodo || it.Priority != apptest.SamplePriorityMedium || len(it.Watchers) != 1 || it.Watchers[0] != "hanako" {
			t.Errorf("%s: status %s priority %s watchers %v", it.Title, it.Status, it.Priority, it.Watchers)
		}
	}
}

func TestCreateEpicFromATemplate(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{
		Name: "リリース", Kind: "Epic", Title: "リリース",
		Children: []workspace.TemplateChild{
			{Kind: "Story", Title: " QA\n 確認 ", Values: map[change.ID]json.RawMessage{
				"assignee": raw(`"taro"`), "systems": raw(`["` + string(apptest.SampleSystemWeb) + `"]`),
			}, Steps: []workspace.TemplateStep{
				{Title: "手順書", Assignee: workspace.StepParent},
				{Title: "記録", Assignee: workspace.StepNone},
			}},
			{Kind: "Task", Title: "本番反映"},
		},
	})
	root, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Kind: "Epic", Title: "11 月のリリース", Template: &tp.ID,
		Values: map[change.ID]json.RawMessage{"assignee": raw(`"hanako"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "Epic「11 月のリリース」ほか 4 件を作成" {
		t.Errorf("summary = %q", got)
	}
	// 番号は深さ優先(02「作成したあと」): Epic 3、Story 4、Step 5・6、Task 7。
	all := byNumber(t, svc)
	qa, s1, s2, deploy := all[4], all[5], all[6], all[7]
	if root.Place.Number != 3 || qa.Title != "QA 確認" || qa.Kind != item.Story || deploy.Kind != item.Task {
		t.Fatalf("numbers: root %d, 4 %+v, 7 %+v", root.Place.Number, qa, deploy)
	}
	if *qa.Place.Parent != root.ID || *deploy.Place.Parent != root.ID || *s1.Place.Parent != qa.ID || *s2.Place.Parent != qa.ID {
		t.Errorf("parents: %v %v %v %v", qa.Place.Parent, deploy.Place.Parent, s1.Place.Parent, s2.Place.Parent)
	}
	if qa.Rank >= deploy.Rank || s1.Rank >= s2.Rank {
		t.Errorf("ranks: %q %q / %q %q", qa.Rank, deploy.Rank, s1.Rank, s2.Rank)
	}
	// 子はテンプレートの値、Step は「親と同じ」なら子の担当者、「未割り当て」なら担当者なし(03「適用するときの規則」)。
	if qa.Assignee != "taro" || len(qa.Systems) != 1 || s1.Assignee != "taro" || s2.Assignee != "" || deploy.Assignee != "" {
		t.Errorf("assignees: qa %q %v, steps %q %q, task %q", qa.Assignee, qa.Systems, s1.Assignee, s2.Assignee, deploy.Assignee)
	}
	checkInitials(t, qa, s1, s2, deploy)
	// 次の番号は、作ったものの次。
	if n, err := svc.NextNumber(t.Context(), store, apptest.SampleWorkspace); err != nil || n != 8 {
		t.Errorf("NextNumber = %d, %v", n, err)
	}
}

func TestCreateTaskFromATemplateTakesTheRootAssigneeForSteps(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{
		Name: "調査", Kind: "Task", Title: "調査",
		Steps: []workspace.TemplateStep{{Title: "まとめる", Assignee: workspace.StepParent}},
	})
	root, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Task", Title: "調査", Template: &tp.ID,
		Values: map[change.ID]json.RawMessage{"assignee": raw(`"taro"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	step := byNumber(t, svc)[root.Place.Number+1]
	if step.Kind != item.Step || *step.Place.Parent != root.ID || step.Assignee != "taro" || last(rec) != "Task「調査」ほか 1 件を作成" {
		t.Errorf("step = %+v, summary %q", step, last(rec))
	}
}

func TestCreateWithAMissingOrMismatchedTemplate(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{Name: "調査", Kind: "Task", Title: "調査"})
	missing := change.ID("01NOSUCHTEMPLATE000000001")
	for name, in := range map[string]items.CreateInput{
		"missing":  {Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Task", Title: "a", Template: &missing},
		"mismatch": {Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Bug", Title: "a", Template: &tp.ID},
	} {
		if _, err := svc.Create(t.Context(), in); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// タイトルの断りが先(02「入力の検査」の表の順。A3b-7)。
	_, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Task", Title: " ", Template: &missing})
	if message(err) != "タイトルを入力してください" {
		t.Errorf("empty title = %v", err)
	}
}
