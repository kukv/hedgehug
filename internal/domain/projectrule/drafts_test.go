package projectrule_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestStatusUseRefusesInitialAndDefault(t *testing.T) {
	both := sample()
	initPending := sample()
	initPending.InitialStatus = pending
	for _, c := range []struct {
		name string
		s    workspace.Settings
		id   change.ID
		want string
	}{
		{"both", both, todo, "初期ステータスで、未着手の既定でもあるため外せません"},
		{"default", both, doing, "進行中の既定のため外せません"},
		{"initial", initPending, pending, "初期ステータスのため外せません"},
	} {
		_, err := projectrule.StatusUseDrafts(c.s, projectrule.Rules{}, p, c.id, false, nil)
		if got := refusalText(t, err); got != c.want {
			t.Errorf("%s: %q", c.name, got)
		}
	}
}

func TestStatusUseRefusesOwnRule(t *testing.T) {
	own := []projectrule.ScopedRule{{Name: "保留で未割り当て", Statuses: []change.ID{pending}}}
	_, err := projectrule.StatusUseDrafts(sample(), projectrule.Rules{}, p, pending, false, own)
	want := "「Pending」はこのプロジェクトだけのルール「保留で未割り当て」が使っているため、使わないことにできません。先にルールを変えてください"
	if got := refusalText(t, err); got != want {
		t.Fatalf("%q", got)
	}
	// 使うことにする側は専用ルールを見ない
	if _, err := projectrule.StatusUseDrafts(sample(), projectrule.Rules{Hidden: []change.ID{pending}}, p, pending, true, own); err != nil {
		t.Fatal(err)
	}
}

func TestStatusUseWritesAddRemove(t *testing.T) {
	f := projectrule.HiddenField(p)
	ds, err := projectrule.StatusUseDrafts(sample(), projectrule.Rules{}, p, pending, false, nil)
	if err != nil || !reflect.DeepEqual(ds, []change.Draft{change.NewAdd(f, pending)}) {
		t.Fatalf("%v %v", ds, err)
	}
	r := projectrule.Rules{Hidden: []change.ID{pending}}
	ds, err = projectrule.StatusUseDrafts(sample(), r, p, pending, true, nil)
	if err != nil || !reflect.DeepEqual(ds, []change.Draft{change.NewRemove(f, pending)}) {
		t.Fatalf("%v %v", ds, err)
	}
	// 同じ状態
	if ds, err = projectrule.StatusUseDrafts(sample(), r, p, pending, false, nil); ds != nil || err != nil {
		t.Fatalf("%v %v", ds, err)
	}
	if ds, err = projectrule.StatusUseDrafts(sample(), projectrule.Rules{}, p, pending, true, nil); ds != nil || err != nil {
		t.Fatalf("%v %v", ds, err)
	}
	if _, err = projectrule.StatusUseDrafts(sample(), projectrule.Rules{}, p, "Nope", false, nil); !errors.Is(err, projectrule.ErrUnknown) {
		t.Fatal(err)
	}
}

func TestWorkflowRefusesCellTheWorkspaceForbids(t *testing.T) {
	s := sample()
	s.Workflow = map[change.ID][]change.ID{todo: {doing}}
	_, err := projectrule.WorkflowDrafts(s, projectrule.Rules{}, p, todo, done, true)
	if got := refusalText(t, err); got != "ワークスペースの制約で変更できません" {
		t.Fatalf("%q", got)
	}
	f := projectrule.WorkflowField(p, todo)
	ds, err := projectrule.WorkflowDrafts(s, projectrule.Rules{}, p, todo, doing, true)
	if err != nil || !reflect.DeepEqual(ds, []change.Draft{change.NewAdd(f, doing)}) {
		t.Fatalf("%v %v", ds, err)
	}
	// 制限のない行は何にでも付けられる
	if _, err := projectrule.WorkflowDrafts(sample(), projectrule.Rules{}, p, todo, done, true); err != nil {
		t.Fatal(err)
	}
	r := projectrule.Rules{Workflow: map[change.ID][]change.ID{todo: {done}}}
	ds, err = projectrule.WorkflowDrafts(s, r, p, todo, done, false)
	if err != nil || !reflect.DeepEqual(ds, []change.Draft{change.NewRemove(f, done)}) {
		t.Fatalf("%v %v", ds, err)
	}
	if ds, err = projectrule.WorkflowDrafts(s, r, p, todo, doing, false); ds != nil || err != nil {
		t.Fatalf("%v %v", ds, err)
	}
	for _, c := range [][2]change.ID{{todo, todo}, {todo, "Nope"}, {"Nope", todo}} {
		if _, err := projectrule.WorkflowDrafts(s, r, p, c[0], c[1], true); !errors.Is(err, projectrule.ErrUnknown) {
			t.Errorf("%v: %v", c, err)
		}
	}
}

func templated() workspace.Settings {
	s := sample()
	s.Templates = []workspace.Template{{ID: "t1", Name: "a", Kind: "Task"}, {ID: "t2", Name: "b", Kind: "Task"}, {ID: "t3", Name: "c", Kind: "Bug"}}
	return s
}

func TestTemplateUseWritesAddRemove(t *testing.T) {
	f := projectrule.TemplatesField(p)
	ds, err := projectrule.TemplateUseDrafts(templated(), projectrule.Rules{}, p, "t1", true)
	if err != nil || !reflect.DeepEqual(ds, []change.Draft{change.NewAdd(f, change.ID("t1"))}) {
		t.Fatalf("%v %v", ds, err)
	}
	r := projectrule.Rules{Templates: []change.ID{"t1"}}
	ds, err = projectrule.TemplateUseDrafts(templated(), r, p, "t1", false)
	if err != nil || !reflect.DeepEqual(ds, []change.Draft{change.NewRemove(f, change.ID("t1"))}) {
		t.Fatalf("%v %v", ds, err)
	}
	if ds, err = projectrule.TemplateUseDrafts(templated(), r, p, "t1", true); ds != nil || err != nil {
		t.Fatalf("%v %v", ds, err)
	}
	if _, err = projectrule.TemplateUseDrafts(templated(), r, p, "zz", true); !errors.Is(err, projectrule.ErrUnknown) {
		t.Fatal(err)
	}
}

func TestTemplateDefaultValues(t *testing.T) {
	s := templated()
	f := projectrule.TemplateDefaultField(p, "Task")
	r := projectrule.Rules{TemplateDefaults: map[string]string{"Task": "t1"}}

	d, ok, err := projectrule.TemplateDefaultDraft(s, r, p, "Task", "")
	if err != nil || !ok || !reflect.DeepEqual(d, change.NewSet(f, "t1", nil)) {
		t.Fatalf("%v %v %v", d, ok, err)
	}
	d, ok, err = projectrule.TemplateDefaultDraft(s, r, p, "Task", projectrule.None)
	if err != nil || !ok || !reflect.DeepEqual(d, change.NewSet(f, "t1", projectrule.None)) {
		t.Fatalf("%v %v %v", d, ok, err)
	}
	d, ok, err = projectrule.TemplateDefaultDraft(s, projectrule.Rules{}, p, "Task", "t2")
	if err != nil || !ok || !reflect.DeepEqual(d, change.NewSet(f, nil, "t2")) {
		t.Fatalf("%v %v %v", d, ok, err)
	}
	// 変わらない
	if _, ok, err = projectrule.TemplateDefaultDraft(s, r, p, "Task", "t1"); ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	if _, ok, err = projectrule.TemplateDefaultDraft(s, projectrule.Rules{}, p, "Task", ""); ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	// 種類が違う・いまない・プロジェクトで使えない
	for _, v := range []string{"t3", "zz"} {
		if _, _, err = projectrule.TemplateDefaultDraft(s, r, p, "Task", v); !errors.Is(err, projectrule.ErrUnknown) {
			t.Errorf("%s: %v", v, err)
		}
	}
	narrow := projectrule.Rules{Templates: []change.ID{"t1"}}
	if _, _, err = projectrule.TemplateDefaultDraft(s, narrow, p, "Task", "t2"); !errors.Is(err, projectrule.ErrUnknown) {
		t.Fatal(err)
	}
}

func TestRuleRunsDraft(t *testing.T) {
	f := projectrule.DisabledField(p)
	d, ok := projectrule.RuleRunsDraft(projectrule.Rules{}, p, "r1", false)
	if !ok || !reflect.DeepEqual(d, change.NewAdd(f, change.ID("r1"))) {
		t.Fatalf("%v %v", d, ok)
	}
	r := projectrule.Rules{Disabled: []change.ID{"r1"}}
	d, ok = projectrule.RuleRunsDraft(r, p, "r1", true)
	if !ok || !reflect.DeepEqual(d, change.NewRemove(f, change.ID("r1"))) {
		t.Fatalf("%v %v", d, ok)
	}
	if _, ok = projectrule.RuleRunsDraft(r, p, "r1", false); ok {
		t.Fatal("同じ状態")
	}
	if _, ok = projectrule.RuleRunsDraft(projectrule.Rules{}, p, "r1", true); ok {
		t.Fatal("同じ状態")
	}
}

func TestSectionLabels(t *testing.T) {
	for sec, want := range map[projectrule.Section]string{
		"statuses": "使うステータス", "workflow": "ワークフローの制約", "templates": "テンプレート", "automation": "自動化",
	} {
		if sec.Label() != want {
			t.Errorf("%s: %s", sec, sec.Label())
		}
	}
}

func TestResetDraftsPerSection(t *testing.T) {
	r := projectrule.Rules{
		Hidden:           []change.ID{pending},
		Workflow:         map[change.ID][]change.ID{todo: {doing, done}, doing: {done}},
		Templates:        []change.ID{"t1"},
		TemplateDefaults: map[string]string{"Task": "t1", "Bug": projectrule.None},
		Disabled:         []change.ID{"r1", "r2"},
	}
	for _, c := range []struct {
		sec  projectrule.Section
		want []change.Draft
	}{
		{"statuses", []change.Draft{change.NewRemove(projectrule.HiddenField(p), pending)}},
		{"workflow", []change.Draft{
			change.NewRemove(projectrule.WorkflowField(p, doing), done),
			change.NewRemove(projectrule.WorkflowField(p, todo), doing),
			change.NewRemove(projectrule.WorkflowField(p, todo), done),
		}},
		{"templates", []change.Draft{
			change.NewRemove(projectrule.TemplatesField(p), change.ID("t1")),
			change.NewSet(projectrule.TemplateDefaultField(p, "Bug"), projectrule.None, nil),
			change.NewSet(projectrule.TemplateDefaultField(p, "Task"), "t1", nil),
		}},
		{"automation", []change.Draft{
			change.NewRemove(projectrule.DisabledField(p), change.ID("r1")),
			change.NewRemove(projectrule.DisabledField(p), change.ID("r2")),
		}},
	} {
		if got := projectrule.ResetDrafts(r, p, c.sec); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v", c.sec, got)
		}
	}
	if got := projectrule.ResetDrafts(projectrule.Rules{}, p, "statuses"); len(got) != 0 {
		t.Fatal(got)
	}
}

// 外した印が残ったまま初期ステータスになった Pending は、戻す操作は通り、外す操作は同じ状態で行なし。
func TestStatusUseHiddenAndNowInitial(t *testing.T) {
	s := sample()
	s.InitialStatus = pending
	r := projectrule.Rules{Hidden: []change.ID{pending}}
	ds, err := projectrule.StatusUseDrafts(s, r, p, pending, true, nil)
	if err != nil || !reflect.DeepEqual(ds, []change.Draft{change.NewRemove(projectrule.HiddenField(p), pending)}) {
		t.Fatalf("%v %v", ds, err)
	}
	if ds, err = projectrule.StatusUseDrafts(s, r, p, pending, false, nil); ds != nil || err != nil {
		t.Fatalf("%v %v", ds, err)
	}
}
