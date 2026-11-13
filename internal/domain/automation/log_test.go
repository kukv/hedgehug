package automation_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// stamp は、drafts をログの続きの行にする(テスト用)。
func stamp(log []change.Record, ds ...change.Draft) []change.Record {
	n := len(log)
	meta := change.Meta{Op: "OP", At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	return append(log, change.Stamp(log, ds, meta, func() change.ID {
		n++
		return change.ID("R" + string(rune('A'+n)))
	})...)
}

func done() automation.Rule {
	return automation.Rule{
		ID: "01RULE2", Name: "完了で外す", Trigger: automation.TriggerStatus, TriggerStatus: "DONE",
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true,
	}
}

func TestRulesRoundTrip(t *testing.T) {
	merged := automation.Rule{
		ID: "01RULE1", Name: "PR マージで完了", Trigger: automation.TriggerPRMerged, AllMerged: true,
		Action: automation.ActionStatus, Value: "DONE", Enabled: true,
	}
	// ワークスペースのログは作成の行から始まる(change.Resolve は作成がないと空を返す)。
	log := stamp(nil, change.NewCreate(map[string]any{"id": "W"}))
	log = stamp(log, automation.Drafts(automation.Rule{}, done())...)
	log = stamp(log, automation.Drafts(automation.Rule{}, merged)...)
	got := automation.FromLog(log)
	if len(got) != 2 || got[0] != merged || got[1] != done() {
		t.Fatalf("FromLog = %+v", got) // 内部 ID の順
	}
	edited := done()
	edited.Name, edited.Enabled = "完了で外す2", false
	ds := automation.Drafts(done(), edited)
	if len(ds) != 2 { // 変わった項目だけ
		t.Errorf("Drafts = %+v", ds)
	}
	log = stamp(log, ds...)
	log = stamp(log, automation.DeleteDraft(merged.ID))
	got = automation.FromLog(log)
	if len(got) != 1 || got[0] != edited {
		t.Errorf("after edit and delete = %+v", got)
	}
	names := automation.Names(log)
	if names[merged.ID] != "PR マージで完了" || names[edited.ID] != "完了で外す2" {
		t.Errorf("Names = %v", names)
	}
}

// 消したルールの削除の行を取り消す restore の行があれば、ルールは戻る(C3a-7)。
func TestRestoredRuleComesBack(t *testing.T) {
	log := stamp(nil, change.NewCreate(map[string]any{"id": "W"}))
	log = stamp(log, automation.Drafts(automation.Rule{}, done())...)
	del := stamp(log, automation.DeleteDraft(done().ID))
	if got := automation.FromLog(del); len(got) != 0 {
		t.Fatalf("deleted rule = %+v", got)
	}
	op := del[len(del)-1].Op
	back := stamp(del, change.NewRestore("rules/"+string(done().ID), op))
	if got := automation.FromLog(back); len(got) != 1 || got[0] != done() {
		t.Errorf("restored rule = %+v", got)
	}
}
