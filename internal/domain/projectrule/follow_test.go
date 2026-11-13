package projectrule_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	pa, pb, pc project.ID = "A", "B", "C"
)

func TestStatusDeleteDrafts(t *testing.T) {
	set := projectrule.Set{
		pb: {
			Hidden:   []change.ID{pending},
			Workflow: map[change.ID][]change.ID{todo: {doing}},
		},
		pa: {
			Hidden: []change.ID{pending, cancel},
			Workflow: map[change.ID][]change.ID{
				pending: {todo, doing},
				todo:    {pending, done},
				doing:   {done},
			},
		},
	}
	got := projectrule.StatusDeleteDrafts(set, pending)
	want := []change.Draft{
		change.NewRemove(projectrule.HiddenField(pa), pending),
		change.NewRemove(projectrule.WorkflowField(pa, pending), todo),
		change.NewRemove(projectrule.WorkflowField(pa, pending), doing),
		change.NewRemove(projectrule.WorkflowField(pa, todo), pending),
		change.NewRemove(projectrule.HiddenField(pb), pending),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := projectrule.StatusDeleteDrafts(set, "Nope"); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

// ワークスペースのルールの削除で、そのルールを外しているプロジェクトの disabled_rules から ID を除く。
func TestRuleDeleteDrafts(t *testing.T) {
	set := projectrule.Set{
		pc: {Disabled: []change.ID{"R1"}},
		pb: {Disabled: []change.ID{"R2"}},
		pa: {Disabled: []change.ID{"R2", "R1"}, Hidden: []change.ID{pending}},
	}
	got := projectrule.RuleDeleteDrafts(set, "R1")
	want := []change.Draft{
		change.NewRemove(projectrule.DisabledField(pa), "R1"),
		change.NewRemove(projectrule.DisabledField(pc), "R1"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := projectrule.RuleDeleteDrafts(set, "Nope"); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestTemplateDropDrafts(t *testing.T) {
	set := projectrule.Set{
		pb: {TemplateDefaults: map[string]string{"Task": "T1", "Bug": "T2", "Epic": "T1"}},
		pa: {
			Templates:        []change.ID{"T2", "T1"},
			TemplateDefaults: map[string]string{"Task": "T1", "Bug": projectrule.None},
		},
		pc: {Templates: []change.ID{"T2"}},
	}
	got := projectrule.TemplateDropDrafts(set, "T1")
	want := []change.Draft{
		change.NewRemove(projectrule.TemplatesField(pa), "T1"),
		change.NewSet(projectrule.TemplateDefaultField(pa, "Task"), "T1", nil),
		change.NewSet(projectrule.TemplateDefaultField(pb, "Epic"), "T1", nil),
		change.NewSet(projectrule.TemplateDefaultField(pb, "Task"), "T1", nil),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestTemplateDeleteImpact(t *testing.T) {
	s := workspace.Settings{Templates: []workspace.Template{{ID: "T1"}, {ID: "T2"}}}
	set := projectrule.Set{
		pc:  {Templates: []change.ID{"T1", "T2"}},
		pb:  {Templates: []change.ID{"T1"}},
		pa:  {TemplateDefaults: map[string]string{"Task": "T1"}},
		"D": {Templates: []change.ID{"T2"}},
		"E": {Templates: []change.ID{"T1", "Gone"}}, // Gone は削除済みのテンプレートの残り
		"F": {TemplateDefaults: map[string]string{"Task": projectrule.None}},
	}
	got := projectrule.TemplateDeleteImpact(s, set, "T1")
	want := projectrule.TemplateImpact{
		Projects:  []project.ID{pa, pb, pc, "E"},
		Unlimited: []project.ID{pb, "E"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := projectrule.TemplateDeleteImpact(s, set, "Nope"); len(got.Projects) != 0 || len(got.Unlimited) != 0 {
		t.Fatalf("%+v", got)
	}
}
