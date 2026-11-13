package projectrule_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// stamp は、drafts をログの続きの行にする(テスト用)。
func stamp(log []change.Record, ds ...change.Draft) []change.Record {
	n := len(log)
	meta := change.Meta{Op: "OP", At: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	return append(log, change.Stamp(log, ds, meta, func() change.ID {
		n++
		return change.ID("R" + string(rune('A'+n)))
	})...)
}

// start は、ワークスペースのログの作成の行(change.Resolve は作成がないと空を返す)。
func start() []change.Record { return stamp(nil, change.NewCreate(map[string]any{"id": "W"})) }

const p project.ID = "P1"

func TestFromLogReadsEachSection(t *testing.T) {
	log := stamp(start(),
		change.NewAdd(projectrule.HiddenField(p), "S2"),
		change.NewAdd(projectrule.WorkflowField(p, "S1"), "S3"),
		change.NewAdd(projectrule.TemplatesField(p), "T1"),
		change.NewSet(projectrule.TemplateDefaultField(p, "Task"), nil, projectrule.None),
		change.NewAdd(projectrule.DisabledField(p), "R1"),
	)
	want := projectrule.Set{p: {
		Hidden:           []change.ID{"S2"},
		Workflow:         map[change.ID][]change.ID{"S1": {"S3"}},
		Templates:        []change.ID{"T1"},
		TemplateDefaults: map[string]string{"Task": "@none"},
		Disabled:         []change.ID{"R1"},
	}}
	got := projectrule.FromLog(log)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FromLog = %+v, want %+v", got, want)
	}
	if got.Of(p).Empty() || !got.Of("other").Empty() {
		t.Error("Of / Empty")
	}
}

func TestFromLogKeepsBothAddsOfOneField(t *testing.T) {
	log := stamp(start(),
		change.NewAdd(projectrule.HiddenField(p), "S2"),
		change.NewAdd(projectrule.HiddenField(p), "S3"),
	)
	if got := projectrule.FromLog(log).Of(p).Hidden; len(got) != 2 {
		t.Errorf("Hidden = %v", got)
	}
}

func TestFromLogDropsRemovedAndEmpty(t *testing.T) {
	log := stamp(start(),
		change.NewAdd(projectrule.HiddenField(p), "S2"),
		change.NewAdd(projectrule.HiddenField("P2"), "S2"),
		change.NewSet(projectrule.TemplateDefaultField(p, "Task"), nil, "T1"),
	)
	log = stamp(log,
		change.NewRemove(projectrule.HiddenField(p), "S2"),
		change.NewSet(projectrule.TemplateDefaultField(p, "Task"), "T1", nil), // null は「設定なし」
	)
	got := projectrule.FromLog(log)
	if _, ok := got[p]; ok {
		t.Errorf("empty project should not be in Set: %+v", got)
	}
	if len(got) != 1 || len(got["P2"].Hidden) != 1 {
		t.Errorf("Set = %+v", got)
	}
}

func TestTemplateDefaultNullIsAbsent(t *testing.T) {
	log := stamp(start(),
		change.NewAdd(projectrule.HiddenField(p), "S2"),
		change.NewSet(projectrule.TemplateDefaultField(p, "Task"), nil, nil),
	)
	if got := projectrule.FromLog(log).Of(p).TemplateDefaults; len(got) != 0 {
		t.Errorf("TemplateDefaults = %v", got)
	}
}

// ワークスペースのログの Multi(workspace.IsMulti)は、プロジェクト別のルールの項目で IsMulti と同じ答えを返す。
func TestIsMultiAgreesWithWorkspace(t *testing.T) {
	cases := map[string]bool{
		projectrule.HiddenField(p):                    true,
		projectrule.WorkflowField(p, "S1"):            true,
		projectrule.TemplatesField(p):                 true,
		projectrule.DisabledField(p):                  true,
		projectrule.TemplateDefaultField(p, "Task"):   false,
		"project_rules/P1/unknown":                    false,
		"project_rules/P1/workflow":                   false,
		"project_rules/P1/hidden_statuses/extra":      false,
		"project_rules/P1/template_default/Task/more": false,
		"prefix": false,
	}
	for f, want := range cases {
		if got := projectrule.IsMulti(f); got != want {
			t.Errorf("projectrule.IsMulti(%q) = %v, want %v", f, got, want)
		}
		if got := workspace.IsMulti(f); got != want {
			t.Errorf("workspace.IsMulti(%q) = %v, want %v", f, got, want)
		}
	}
}
