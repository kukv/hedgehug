package projectrule_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	todo, doing, done, pending, cancel change.ID = "Todo", "InProgress", "Done", "Pending", "Cancel"
)

// sample は、既定のサンプルと同じ 5 つ(Todo が初期。Pending は分類の既定でない)。
func sample() workspace.Settings {
	v := func(id change.ID, c workspace.Category) workspace.Value {
		return workspace.Value{ID: id, Name: string(id), Category: c}
	}
	return workspace.Settings{
		Statuses: []workspace.Value{
			v(todo, workspace.CategoryTodo), v(doing, workspace.CategoryDoing), v(done, workspace.CategoryDone),
			v(pending, workspace.CategoryDoing), v(cancel, workspace.CategoryCanceled),
		},
		InitialStatus: todo,
		CategoryDefaults: map[workspace.Category]change.ID{
			workspace.CategoryTodo: todo, workspace.CategoryDoing: doing,
			workspace.CategoryDone: done, workspace.CategoryCanceled: cancel,
		},
	}
}

func refusalText(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("断られるはずが通った")
	}
	return err.Error()
}

func TestUsesKeepsInitialAndDefaults(t *testing.T) {
	s := sample()
	r := projectrule.Rules{Hidden: []change.ID{todo, pending}}
	if !projectrule.Uses(s, r, todo) || projectrule.Uses(s, r, pending) {
		t.Fatal("初期ステータスは使い、既定でない Pending は使わない")
	}
	s.InitialStatus = doing
	s.CategoryDefaults[workspace.CategoryTodo] = doing
	if projectrule.Uses(s, r, todo) {
		t.Fatal("初期でも既定でもなくなった Todo は外れる")
	}
}

func TestCheckUseText(t *testing.T) {
	r := projectrule.Rules{Hidden: []change.ID{pending}}
	if got := refusalText(t, projectrule.CheckUse(sample(), r, pending)); got != "このプロジェクトでは「Pending」を使いません" {
		t.Fatalf("文: %q", got)
	}
	if err := projectrule.CheckUse(sample(), r, todo); err != nil {
		t.Fatal(err)
	}
}

func TestUsedStatusesInOrder(t *testing.T) {
	r := projectrule.Rules{Hidden: []change.ID{pending, doing}}
	var got []change.ID
	for _, v := range projectrule.UsedStatuses(sample(), r) {
		got = append(got, v.ID)
	}
	// InProgress は分類の既定なので外れない
	if want := []change.ID{todo, doing, done, cancel}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCheckTransitionBothMustAllow(t *testing.T) {
	s := sample()
	s.Workflow = map[change.ID][]change.ID{todo: {doing, done}}
	r := projectrule.Rules{Workflow: map[change.ID][]change.ID{todo: {doing}}}
	want := "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)"
	if got := refusalText(t, projectrule.CheckTransition(s, r, todo, done)); got != want {
		t.Fatalf("got %q", got)
	}
	if err := projectrule.CheckTransition(s, r, todo, doing); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTransitionProjectOnly(t *testing.T) {
	r := projectrule.Rules{Workflow: map[change.ID][]change.ID{todo: {doing}}}
	want := "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)"
	if got := refusalText(t, projectrule.CheckTransition(sample(), r, todo, done)); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestCheckTransitionNoCommonTarget(t *testing.T) {
	s := sample()
	s.Workflow = map[change.ID][]change.ID{todo: {done}}
	r := projectrule.Rules{Workflow: map[change.ID][]change.ID{todo: {doing}}}
	want := "ワークフローの制約: Todo から Done には変更できません(変更できる先がありません)"
	if got := refusalText(t, projectrule.CheckTransition(s, r, todo, done)); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestProjectRowOfUnusedOnlyIsUnrestricted(t *testing.T) {
	r := projectrule.Rules{Hidden: []change.ID{pending}, Workflow: map[change.ID][]change.ID{todo: {pending}}}
	if projectrule.ProjectRowRestricted(sample(), r, todo) {
		t.Fatal("使わないステータスだけの行は制限でない")
	}
	if err := projectrule.CheckTransition(sample(), r, todo, done); err != nil {
		t.Fatal(err)
	}
	r.Workflow[todo] = []change.ID{doing}
	if !projectrule.ProjectRowRestricted(sample(), r, todo) {
		t.Fatal("使うステータスのチェックがあれば制限")
	}
}

func TestAllowedDropsUnused(t *testing.T) {
	s := sample()
	s.Workflow = map[change.ID][]change.ID{todo: {doing, pending}}
	r := projectrule.Rules{Hidden: []change.ID{pending}}
	to, restricted := projectrule.Allowed(s, r, todo)
	if !reflect.DeepEqual(to, []change.ID{doing}) || !restricted {
		t.Fatalf("got %v %v", to, restricted)
	}
	to, restricted = projectrule.Allowed(sample(), r, todo)
	if restricted || len(to) != 3 {
		t.Fatalf("制限なしは使う先すべて(from 以外): %v %v", to, restricted)
	}
}

func TestCheckTransitionPassesMissingFrom(t *testing.T) {
	s := sample()
	s.Workflow = map[change.ID][]change.ID{"Gone": {done}}
	r := projectrule.Rules{Workflow: map[change.ID][]change.ID{"Gone": {doing}}}
	if err := projectrule.CheckTransition(s, r, "Gone", cancel); err != nil {
		t.Fatal(err)
	}
	if err := projectrule.CheckTransition(s, r, "", cancel); err != nil {
		t.Fatal(err)
	}
	if err := projectrule.CheckTransition(s, r, todo, todo); err != nil {
		t.Fatal(err)
	}
}

func TestUsableTemplatesAndDefault(t *testing.T) {
	s := sample()
	s.Templates = []workspace.Template{{ID: "T1", Kind: "Task"}, {ID: "T2", Kind: "Task"}, {ID: "T3", Kind: "Bug"}}
	s.TemplateDefaults = map[string]change.ID{"Task": "T2"}
	ids := func(ts []workspace.Template) []change.ID {
		var out []change.ID
		for _, x := range ts {
			out = append(out, x.ID)
		}
		return out
	}
	if got := ids(projectrule.UsableTemplates(s, projectrule.Rules{}, "Task")); !reflect.DeepEqual(got, []change.ID{"T1", "T2"}) {
		t.Fatalf("制限なし: %v", got)
	}
	if id, ok := projectrule.DefaultTemplate(s, projectrule.Rules{}, "Task"); !ok || id != "T2" {
		t.Fatalf("ワークスペースの既定: %v %v", id, ok)
	}
	r := projectrule.Rules{Templates: []change.ID{"T1"}}
	if got := ids(projectrule.UsableTemplates(s, r, "Task")); !reflect.DeepEqual(got, []change.ID{"T1"}) {
		t.Fatalf("絞る: %v", got)
	}
	if _, ok := projectrule.DefaultTemplate(s, r, "Task"); ok {
		t.Fatal("T2 が使えないので既定なし")
	}
	r.TemplateDefaults = map[string]string{"Task": "T1"}
	if id, ok := projectrule.DefaultTemplate(s, r, "Task"); !ok || id != "T1" {
		t.Fatalf("プロジェクトの既定: %v %v", id, ok)
	}
	r.TemplateDefaults = map[string]string{"Task": "T3"} // 使えない(種類違い)
	if _, ok := projectrule.DefaultTemplate(s, r, "Task"); ok {
		t.Fatal("使えない既定はワークスペースの既定も使えず、なし")
	}
	r.Templates = nil
	if id, ok := projectrule.DefaultTemplate(s, r, "Task"); ok || id != "" {
		t.Fatal("使えない既定は、ワークスペースの既定があってもなし")
	}
	r.TemplateDefaults = nil
	if id, ok := projectrule.DefaultTemplate(s, r, "Task"); !ok || id != "T2" {
		t.Fatalf("未設定ならワークスペースの既定: %v %v", id, ok)
	}
	r = projectrule.Rules{TemplateDefaults: map[string]string{"Task": projectrule.None}}
	if _, ok := projectrule.DefaultTemplate(s, r, "Task"); ok {
		t.Fatal("@none はなし")
	}
	r = projectrule.Rules{Templates: []change.ID{"Gone"}}
	if got := ids(projectrule.UsableTemplates(s, r, "Task")); !reflect.DeepEqual(got, []change.ID{"T1", "T2"}) {
		t.Fatalf("消えたテンプレートだけなら全部: %v", got)
	}
}
