package automation_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
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
		if !automation.Valid(s, logins, r) {
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
		if automation.Valid(s, logins, r) {
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
