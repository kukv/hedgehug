package automation_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestStatusDeleteRewritesRules(t *testing.T) {
	rules := []automation.Rule{
		rule("R1", automation.TriggerStatus, "REVIEW", automation.ActionAssignee, "taro"), // きっかけを書き換える
		rule("R2", automation.TriggerPROpen, "", automation.ActionStatus, "REVIEW"),       // することを書き換える
		rule("R3", automation.TriggerStatus, "TODO", automation.ActionStatus, "REVIEW"),   // 同じになる: 無効
		rule("R4", automation.TriggerStatus, "DONE", automation.ActionStatus, "TODO"),     // 関係ない
	}
	ds, changes := automation.StatusDeleteDrafts(rules, "REVIEW", "TODO")
	if len(changes) != 3 {
		t.Fatalf("changes = %+v", changes)
	}
	if changes[0].Part != "きっかけ" || changes[0].Rule.TriggerStatus != "TODO" || changes[0].Disabled {
		t.Errorf("R1 = %+v", changes[0])
	}
	if changes[1].Part != "すること" || changes[1].Rule.Value != "TODO" || changes[1].Disabled {
		t.Errorf("R2 = %+v", changes[1])
	}
	if !changes[2].Disabled || changes[2].Rule.Enabled || changes[2].Rule.Value != "TODO" {
		t.Errorf("R3 = %+v", changes[2])
	}
	if len(ds) != 4 { // R1: trigger_status、R2: value、R3: value と enabled
		t.Errorf("drafts = %+v", ds)
	}
}

func TestStatusDeleteAlreadyDisabledRuleIsNotReportedDisabled(t *testing.T) {
	r := rule("R1", automation.TriggerStatus, "TODO", automation.ActionStatus, "REVIEW")
	r.Enabled = false
	ds, changes := automation.StatusDeleteDrafts([]automation.Rule{r}, "REVIEW", "TODO")
	if len(changes) != 1 || changes[0].Disabled || changes[0].Rule.Enabled {
		t.Errorf("changes = %+v", changes)
	}
	if len(ds) != 1 { // value だけ。enabled の行は書かない
		t.Errorf("drafts = %+v", ds)
	}
}

func statusDeleteRows(extra ...change.Draft) []change.Record {
	ds := append([]change.Draft{change.NewDelete("statuses/s1")}, extra...)
	return change.Stamp(nil, ds, change.Meta{Op: "OP1"}, func() change.ID { return "R" })
}

func TestFollowedFromRowsRewritten(t *testing.T) {
	rules := []automation.Rule{rule("r1", automation.TriggerStatus, "s2", automation.ActionAssignee, "taro")}
	rows := statusDeleteRows(change.NewSet(automation.Field("r1", "trigger_status"), "s1", "s2"))
	deleted, changes := automation.FollowedFromRows(rows, rules)
	if deleted != "s1" || len(changes) != 1 {
		t.Fatalf("deleted = %q, changes = %+v", deleted, changes)
	}
	if c := changes[0]; c.Part != "きっかけ" || c.Disabled || c.Rule.ID != "r1" {
		t.Errorf("change = %+v", c)
	}
}

func TestFollowedFromRowsDisabled(t *testing.T) {
	rules := []automation.Rule{rule("r1", automation.TriggerStatus, "s2", automation.ActionStatus, "s2")}
	rows := statusDeleteRows(
		change.NewSet(automation.Field("r1", "value"), "s1", "s2"),
		change.NewSet(automation.Field("r1", "enabled"), true, false),
	)
	_, changes := automation.FollowedFromRows(rows, rules)
	if len(changes) != 1 || !changes[0].Disabled || changes[0].Part != "すること" {
		t.Errorf("changes = %+v", changes)
	}
}

func TestFollowedFromRowsBoth(t *testing.T) {
	rules := []automation.Rule{rule("r1", automation.TriggerStatus, "s2", automation.ActionStatus, "s2")}
	rows := statusDeleteRows(
		change.NewSet(automation.Field("r1", "trigger_status"), "s1", "s2"),
		change.NewSet(automation.Field("r1", "value"), "s1", "s2"),
	)
	_, changes := automation.FollowedFromRows(rows, rules)
	if len(changes) != 1 || changes[0].Part != "すること" {
		t.Errorf("changes = %+v", changes)
	}
}

func TestFollowedFromRowsNoDelete(t *testing.T) {
	rules := []automation.Rule{rule("r1", automation.TriggerStatus, "s2", automation.ActionAssignee, "taro")}
	rows := change.Stamp(nil, []change.Draft{change.NewSet(automation.Field("r1", "trigger_status"), "s1", "s2")},
		change.Meta{Op: "OP1"}, func() change.ID { return "R" })
	deleted, changes := automation.FollowedFromRows(rows, rules)
	if deleted != "" || len(changes) != 0 {
		t.Errorf("deleted = %q, changes = %+v", deleted, changes)
	}
}
