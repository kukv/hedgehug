package repair_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

func scopedRule(id string, scope project.ID) automation.Rule {
	return automation.Rule{
		ID: change.ID(id), Name: "rule " + id, Trigger: automation.TriggerStatus, TriggerStatus: "S1",
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true, Scope: scope,
	}
}

// ownerLog は、R1 が P2 の専用ルール、R9 がワークスペース全体のルールのログ。
func ownerLog() []change.Record {
	log := put(wsBase(), "o1", day, "taro", automation.Drafts(automation.Rule{}, scopedRule("R1", "P2"))...)
	return put(log, "o2", day, "taro", automation.Drafts(automation.Rule{}, scopedRule("R9", ""))...)
}

func rec(t change.Type, f string) change.Record { return change.Record{T: t, F: f} }

func TestRuleOwner(t *testing.T) {
	scopes := repair.RuleScopes(ownerLog())
	tests := []struct {
		name string
		r    change.Record
		want project.ID
		ok   bool
	}{
		{"プロジェクト別のルールの add", rec(change.Add, "project_rules/P1/hidden_statuses"), "P1", true},
		{"プロジェクト別の既定テンプレートの set", rec(change.Set, "project_rules/P1/template_default/Task"), "P1", true},
		{"専用ルールの項目の set", rec(change.Set, "rules/R1/name"), "P2", true},
		{"専用ルールの delete", rec(change.Delete, "rules/R1"), "P2", true},
		{"scope のないルールには持ち主がない", rec(change.Set, "rules/R9/name"), "", false},
		{"ルールの行がない項目には持ち主がない", rec(change.Set, "initial_status"), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := repair.RuleOwner(tt.r, scopes)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// p1Rules は、P1 にすべての区画のルールを持たせる。
func p1Rules(log []change.Record) []change.Record {
	return put(log, "p1", day, "taro",
		change.NewAdd(projectrule.HiddenField("P1"), "S1"),
		change.NewAdd(projectrule.WorkflowField("P1", "S1"), "S2"),
		change.NewAdd(projectrule.TemplatesField("P1"), "T1"),
		change.NewSet(projectrule.TemplateDefaultField("P1", "Task"), nil, "T1"),
		change.NewAdd(projectrule.DisabledField("P1"), "R9"),
	)
}

func notP2(p project.ID) bool { return p == "P2" }

func TestOrphans(t *testing.T) {
	log := put(wsBase(), "o1", day, "taro", automation.Drafts(automation.Rule{}, scopedRule("R1", "P1"))...)
	log = put(log, "o2", day, "taro", automation.Drafts(automation.Rule{}, scopedRule("R2", "P2"))...)
	log = put(p1Rules(log), "p2", day, "taro", change.NewAdd(projectrule.DisabledField("P2"), "R9"))
	got := repair.Orphans(log, notP2)
	want := []repair.OrphanRules{{Project: "P1", Drafts: []change.Draft{
		change.NewRemove(projectrule.HiddenField("P1"), "S1"),
		change.NewRemove(projectrule.WorkflowField("P1", "S1"), "S2"),
		change.NewRemove(projectrule.TemplatesField("P1"), "T1"),
		change.NewSet(projectrule.TemplateDefaultField("P1", "Task"), "T1", nil),
		change.NewRemove(projectrule.DisabledField("P1"), "R9"),
		automation.DeleteDraft("R1"),
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestOrphansOrderedByProject(t *testing.T) {
	log := put(wsBase(), "p", day, "taro",
		change.NewAdd(projectrule.HiddenField("P3"), "S1"), change.NewAdd(projectrule.HiddenField("P1"), "S1"))
	got := repair.Orphans(log, func(project.ID) bool { return false })
	if len(got) != 2 || got[0].Project != "P1" || got[1].Project != "P3" {
		t.Fatalf("プロジェクトの内部 ID の順: %+v", got)
	}
}

func TestOrphanRulesKeepsDeletedProject(t *testing.T) {
	log := put(p1Rules(wsBase()), "o1", day, "taro", automation.Drafts(automation.Rule{}, scopedRule("R1", "P1"))...)
	if got := repair.Orphans(log, func(project.ID) bool { return true }); len(got) != 0 {
		t.Fatalf("削除済みのプロジェクトはワークスペースにあるものとして数える: %+v", got)
	}
}

func TestOrphanRulesDraftsDisabledAndOwn(t *testing.T) {
	log := put(wsBase(), "o1", day, "taro", automation.Drafts(automation.Rule{}, scopedRule("R9", ""))...)
	log = put(log, "o2", day, "taro", automation.Drafts(automation.Rule{}, scopedRule("R2", "P1"))...)
	log = put(log, "o3", day, "taro", automation.DeleteDraft("R2"))
	log = put(log, "p1", day, "taro", change.NewAdd(projectrule.DisabledField("P1"), "R9"))
	got := repair.Orphans(log, notP2)
	want := []repair.OrphanRules{{Project: "P1", Drafts: []change.Draft{
		change.NewRemove(projectrule.DisabledField("P1"), "R9"),
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ワークスペース全体のルールは外すだけで消さず、削除済みの専用ルールは消さない: %+v", got)
	}
}

func TestOrphansNothingLeft(t *testing.T) {
	log := put(p1Rules(wsBase()), "p2", day, "taro",
		change.NewRemove(projectrule.HiddenField("P1"), "S1"),
		change.NewRemove(projectrule.WorkflowField("P1", "S1"), "S2"),
		change.NewRemove(projectrule.TemplatesField("P1"), "T1"),
		change.NewSet(projectrule.TemplateDefaultField("P1", "Task"), "T1", nil),
		change.NewRemove(projectrule.DisabledField("P1"), "R9"),
	)
	if rules := projectrule.FromLog(log); len(rules) != 0 {
		t.Fatalf("前提: ルールは空: %+v", rules)
	}
	if got := repair.Orphans(log, notP2); len(got) != 0 {
		t.Fatalf("残っている行がなければ入れない: %+v", got)
	}
}
