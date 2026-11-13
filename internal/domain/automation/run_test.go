package automation_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// runSettings は、ステータス 4 つと、Task・Step だけが使う担当者を持つ設定。
func runSettings() workspace.Settings {
	s := settings()
	s.Properties = []workspace.Property{
		{ID: "status", Key: "status", Name: "ステータス", Type: workspace.TypeStatus, Builtin: true, Kinds: workspace.Kinds},
		{ID: "assignee", Key: "assignee", Name: "担当者", Type: workspace.TypeUser, Builtin: true, Kinds: []string{"Task", "Step"}},
	}
	return s
}

func rule(id string, trig automation.Trigger, trigStatus change.ID, act automation.Action, v string) automation.Rule {
	return automation.Rule{ID: change.ID(id), Name: id, Trigger: trig, TriggerStatus: trigStatus, Action: act, Value: v, Enabled: true}
}

func TestRunAppliesMatchingRulesInOrder(t *testing.T) {
	s := runSettings()
	rules := []automation.Rule{
		rule("R1", automation.TriggerStatus, "DONE", automation.ActionAssignee, automation.Unassigned),
		rule("R2", automation.TriggerStatus, "DOING", automation.ActionAssignee, "taro"), // きっかけが違う
		rule("R3", automation.TriggerStatus, "DONE", automation.ActionAssignee, "taro"),
	}
	rules[0].Enabled = true
	off := rule("R4", automation.TriggerStatus, "DONE", automation.ActionStatus, "CANCEL")
	off.Enabled = false
	rules = append(rules, off)
	it := item.Item{ID: "I1", Kind: item.Task, Status: "DONE", Assignee: "hanako"}
	got := automation.Run(s, rules, it, automation.Event{Trigger: automation.TriggerStatus, Status: "DONE"})
	if len(got) != 2 || got[0].Rule.ID != "R1" || got[1].Rule.ID != "R3" {
		t.Fatalf("Run = %+v", got)
	}
	// R3 は R1 が外したあとの値(なし)から taro にする
	if got[1].Draft == nil || got[1].Draft.Rule != "R3" || string(got[1].Draft.From) != `null` || string(got[1].To) != `"taro"` {
		t.Errorf("R3 = %+v / %+v", got[1], got[1].Draft)
	}
}

func TestRunSkipsRulesThatChangeNothing(t *testing.T) {
	s := runSettings()
	rules := []automation.Rule{
		rule("R1", automation.TriggerStepsDone, "", automation.ActionAssignee, "taro"),
		rule("R2", automation.TriggerStepsDone, "", automation.ActionStatus, "DONE"),
	}
	epic := item.Item{ID: "E1", Kind: item.Epic, Status: "DONE"} // Epic は担当者を使わない・すでに Done
	if got := automation.Run(s, rules, epic, automation.Event{Trigger: automation.TriggerStepsDone}); len(got) != 0 {
		t.Errorf("Run = %+v", got)
	}
	task := item.Item{ID: "T1", Kind: item.Task, Status: "DOING", Assignee: "taro"}
	got := automation.Run(s, rules, task, automation.Event{Trigger: automation.TriggerStepsDone})
	if len(got) != 1 || got[0].Rule.ID != "R2" {
		t.Errorf("Run = %+v", got)
	}
}

func TestRunStopsOnTheWorkflowAndGoesOn(t *testing.T) {
	s := runSettings()
	s.Workflow = map[change.ID][]change.ID{"TODO": {"DOING"}}
	rules := []automation.Rule{
		rule("R1", automation.TriggerPROpen, "", automation.ActionStatus, "DONE"),
		rule("R2", automation.TriggerPROpen, "", automation.ActionStatus, "DOING"),
		rule("R3", automation.TriggerPROpen, "", automation.ActionStatus, "DONE"), // DOING からは通る
	}
	it := item.Item{ID: "T1", Kind: item.Task, Status: "TODO"}
	got := automation.Run(s, rules, it, automation.Event{Trigger: automation.TriggerPROpen})
	if len(got) != 3 {
		t.Fatalf("Run = %+v", got)
	}
	if got[0].Draft != nil || got[0].Stopped != "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)" || string(got[0].To) != `"DONE"` {
		t.Errorf("R1 = %+v", got[0])
	}
	if got[1].Draft == nil || got[2].Draft == nil || string(got[2].Draft.From) != `"DOING"` {
		t.Errorf("R2, R3 = %+v, %+v", got[1], got[2])
	}
}

func TestRunMergedCondition(t *testing.T) {
	s := runSettings()
	r := rule("R1", automation.TriggerPRMerged, "", automation.ActionStatus, "DONE")
	r.AllMerged = true
	it := item.Item{ID: "T1", Kind: item.Task, Status: "DOING"}
	if got := automation.Run(s, []automation.Rule{r}, it, automation.Event{Trigger: automation.TriggerPRMerged}); len(got) != 0 {
		t.Errorf("not all merged: %+v", got)
	}
	if got := automation.Run(s, []automation.Rule{r}, it, automation.Event{Trigger: automation.TriggerPRMerged, AllMerged: true}); len(got) != 1 {
		t.Errorf("all merged: %+v", got)
	}
}

func TestStepsDone(t *testing.T) {
	s := settings()
	steps := []item.Item{{ID: "S1", Status: "DONE"}, {ID: "S2", Status: "CANCEL"}, {ID: "S3", Status: "DONE"}}
	if !automation.StepsDone(s, steps, map[item.ID]change.ID{"S3": "TODO"}) {
		t.Error("the last step completes the parent")
	}
	if automation.StepsDone(s, steps, map[item.ID]change.ID{"S3": "CANCEL"}) {
		t.Error("already all closed before the change")
	}
	steps[0].Status = "DOING"
	if automation.StepsDone(s, steps, map[item.ID]change.ID{"S3": "TODO"}) {
		t.Error("a step is still open")
	}
	if automation.StepsDone(s, nil, nil) {
		t.Error("no steps")
	}
}

func TestRunAllRunsInListOrderAcrossEvents(t *testing.T) {
	s := runSettings()
	rules := []automation.Rule{
		rule("R1", automation.TriggerReview, "", automation.ActionStatus, "DOING"),
		rule("R2", automation.TriggerStatus, "DONE", automation.ActionStatus, "CANCEL"), // どのきっかけにも合わない
		rule("R3", automation.TriggerPROpen, "", automation.ActionStatus, "DONE"),
	}
	it := item.Item{ID: "T1", Kind: item.Task, Status: "TODO"}
	evs := []automation.Event{{Trigger: automation.TriggerPROpen}, {Trigger: automation.TriggerReview}}
	got := automation.RunAll(s, rules, it, evs)
	if len(got) != 2 || got[0].Rule.ID != "R1" || got[1].Rule.ID != "R3" {
		t.Fatalf("RunAll = %+v, want R1 then R3 (list order, not event order)", got)
	}
	// R3 は R1 が変えたあとの InProgress から判定する
	if got[1].Draft == nil || string(got[1].Draft.From) != `"DOING"` {
		t.Errorf("R3 = %+v / %+v", got[1], got[1].Draft)
	}
}

func TestRunAllLaterRuleJudgesFromTheOldStatusWhenStopped(t *testing.T) {
	s := runSettings()
	s.Workflow = map[change.ID][]change.ID{"TODO": {"DOING"}}
	rules := []automation.Rule{
		rule("R1", automation.TriggerReview, "", automation.ActionStatus, "DONE"),  // Todo からは止まる
		rule("R2", automation.TriggerPROpen, "", automation.ActionStatus, "DOING"), // Todo から通る
	}
	it := item.Item{ID: "T1", Kind: item.Task, Status: "TODO"}
	evs := []automation.Event{{Trigger: automation.TriggerPROpen}, {Trigger: automation.TriggerReview}}
	got := automation.RunAll(s, rules, it, evs)
	if len(got) != 2 || got[0].Draft != nil || got[0].Stopped == "" {
		t.Fatalf("RunAll = %+v", got)
	}
	if got[1].Draft == nil || string(got[1].Draft.From) != `"TODO"` {
		t.Errorf("R2 = %+v / %+v, want it judged from Todo", got[1], got[1].Draft)
	}
}
