package automation_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func settings() workspace.Settings {
	return workspace.Settings{Statuses: []workspace.Value{
		{ID: "TODO", Name: "Todo", Category: workspace.CategoryTodo},
		{ID: "DOING", Name: "InProgress", Category: workspace.CategoryDoing},
		{ID: "DONE", Name: "Done", Category: workspace.CategoryDone},
		{ID: "CANCEL", Name: "Cancel", Category: workspace.CategoryCanceled},
	}}
}

func TestNormalizeDropsUnusedFields(t *testing.T) {
	r := automation.Normalize(automation.Rule{Name: "  a  ", Trigger: automation.TriggerPROpen, TriggerStatus: "DONE", AllMerged: true})
	if r.Name != "a" || r.TriggerStatus != "" || r.AllMerged {
		t.Errorf("Normalize = %+v", r)
	}
}

func TestValid(t *testing.T) {
	s, logins := settings(), []string{"taro"}
	ok := []automation.Rule{
		{Trigger: automation.TriggerStatus, TriggerStatus: "DONE", Action: automation.ActionAssignee, Value: automation.Unassigned},
		{Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus, Value: "DONE"},
		{Trigger: automation.TriggerStepsDone, Action: automation.ActionAssignee, Value: "Taro"}, // ログイン名は大文字と小文字を区別しない
	}
	for _, r := range ok {
		if !automation.Valid(s, nil, logins, automation.Rule{}, r) {
			t.Errorf("Valid(%+v) = false", r)
		}
	}
	bad := []automation.Rule{
		{Trigger: "nope", Action: automation.ActionStatus, Value: "DONE"},
		{Trigger: automation.TriggerStatus, TriggerStatus: "GONE", Action: automation.ActionStatus, Value: "DONE"},
		{Trigger: automation.TriggerPROpen, Action: automation.ActionStatus, Value: "GONE"},
		{Trigger: automation.TriggerPROpen, Action: automation.ActionAssignee, Value: "jiro"},
		{Trigger: automation.TriggerPROpen, Action: "nope", Value: "DONE"},
	}
	for _, r := range bad {
		if automation.Valid(s, nil, logins, automation.Rule{}, r) {
			t.Errorf("Valid(%+v) = true", r)
		}
	}
}

func TestCheck(t *testing.T) {
	rules := []automation.Rule{{ID: "R1", Name: "完了で外す"}}
	cases := []struct {
		r    automation.Rule
		want string
	}{
		{automation.Rule{ID: "R2", Name: ""}, "名前を入力してください"},
		{automation.Rule{ID: "R2", Name: "完了で外す"}, "同じ名前のルールがあります"},
		{
			automation.Rule{ID: "R2", Name: "x", Trigger: automation.TriggerStatus, TriggerStatus: "DONE", Action: automation.ActionStatus, Value: "DONE"},
			"きっかけと同じステータスには変更できません",
		},
		{automation.Rule{ID: "R1", Name: "完了で外す"}, ""}, // 自分の名前は重複にしない
	}
	for _, c := range cases {
		if got := message(automation.Check(rules, c.r)); got != c.want {
			t.Errorf("Check(%+v) = %q, want %q", c.r, got, c.want)
		}
	}
}

func TestTexts(t *testing.T) {
	c := automation.ChangeText("ステータス", "Done")
	got := []string{
		c,
		automation.ActivityText("PR マージで完了", c),
		automation.AppliedText("PR マージで完了", c),
		automation.StoppedText("PR マージで完了"),
		automation.IgnoreText("PR マージで完了"),
		automation.DisabledByDeleteText("X"),
		automation.RewrittenText("X", "Review", "すること", "Todo"),
	}
	want := []string{
		"ステータスを Done に変更",
		"「PR マージで完了」でステータスを Done に変更",
		"「PR マージで完了」でステータスを Done に変更しました",
		"「PR マージで完了」が制約で止まりました",
		"「PR マージで完了」の変更を制約を無視して適用",
		"自動化「X」は、ステータスの削除で、きっかけと同じステータスへの変更になったため、無効にしました",
		"自動化「X」は、ステータス「Review」の削除で、することのステータスを「Todo」に書き換えました",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValidRejectsUnusedStatusForScopedRule(t *testing.T) {
	s := settings()
	s.InitialStatus = "TODO"
	set := projectrule.Set{"P": {Hidden: []change.ID{"CANCEL"}}}
	scoped := func(p string, r automation.Rule) automation.Rule { r.Scope = project.ID(p); return r }
	toCancel := automation.Rule{Trigger: automation.TriggerPROpen, Action: automation.ActionStatus, Value: "CANCEL"}
	fromCancel := automation.Rule{Trigger: automation.TriggerStatus, TriggerStatus: "CANCEL", Action: automation.ActionAssignee, Value: automation.Unassigned}
	for _, r := range []automation.Rule{toCancel, fromCancel} {
		if automation.Valid(s, set, nil, automation.Rule{}, scoped("P", r)) {
			t.Errorf("使わないステータスの専用ルールが通った: %+v", r)
		}
		if !automation.Valid(s, set, nil, automation.Rule{}, scoped("Q", r)) || !automation.Valid(s, set, nil, automation.Rule{}, r) {
			t.Errorf("ほかのプロジェクト・全体のルールは通る: %+v", r)
		}
	}
	got := automation.Statuses(automation.Rule{Trigger: automation.TriggerStatus, TriggerStatus: "A", Action: automation.ActionStatus, Value: "B"})
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("Statuses = %v", got)
	}
	if got := automation.Statuses(automation.Rule{Trigger: automation.TriggerPROpen, Action: automation.ActionAssignee, Value: "x"}); len(got) != 0 {
		t.Errorf("Statuses = %v", got)
	}
}

// 編集では、変えたきっかけ・することのステータスだけを、使うものか確かめる。ステータスの削除の書き換えなどで
// 使わないステータスを指したままのルールも、そのまま改名できる。
func TestValidChecksOnlyChangedStatusesOfScopedRule(t *testing.T) {
	s := settings()
	s.InitialStatus = "TODO"
	set := projectrule.Set{"P": {Hidden: []change.ID{"CANCEL"}}}
	old := automation.Rule{Trigger: automation.TriggerPROpen, Action: automation.ActionStatus, Value: "CANCEL", Scope: "P"}
	renamed := old
	renamed.Name = "改名"
	if !automation.Valid(s, set, nil, old, renamed) {
		t.Error("変えていない使わないステータスで、改名が断られた")
	}
	addTrigger := old
	addTrigger.Trigger, addTrigger.TriggerStatus = automation.TriggerStatus, "CANCEL"
	if automation.Valid(s, set, nil, old, addTrigger) {
		t.Error("きっかけを使わないステータスに変えたのに通った")
	}
	fromTrigger := automation.Rule{Trigger: automation.TriggerStatus, TriggerStatus: "CANCEL", Action: automation.ActionAssignee, Value: automation.Unassigned, Scope: "P"}
	toStatus := fromTrigger
	toStatus.Action, toStatus.Value = automation.ActionStatus, "CANCEL"
	if automation.Valid(s, set, nil, fromTrigger, toStatus) {
		t.Error("することを使わないステータスに変えたのに通った")
	}
}

// 集合にないプロジェクトの専用ルールは、そのプロジェクトの規則がないものとして、使うステータスを通す。
func TestValidScopedToProjectAbsentFromSet(t *testing.T) {
	s := settings()
	r := automation.Rule{Trigger: automation.TriggerPROpen, Action: automation.ActionStatus, Value: "DONE", Scope: "Z"}
	if !automation.Valid(s, projectrule.Set{"P": {Hidden: []change.ID{"DONE"}}}, nil, automation.Rule{}, r) || !automation.Valid(s, nil, nil, automation.Rule{}, r) {
		t.Errorf("Valid(%+v) = false", r)
	}
}
