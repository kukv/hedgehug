package automation_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
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
