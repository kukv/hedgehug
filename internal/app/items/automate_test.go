package items_test

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

// setupAuto は、ルールを書いた Store と、通知を覚える受信箱。
func setupAuto(t *testing.T, rules ...automation.Rule) (items.Service, *recorder, *apptest.Inbox) {
	t.Helper()
	svc, rec := setupRec(t)
	inbox := &apptest.Inbox{}
	svc.Inbox = inbox
	op := port.Operation{Actor: change.User("taro"), Summary: "ルールを作成"}
	err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		for _, r := range rules {
			if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(automation.Rule{}, r)...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, rec, inbox
}

func autoRule(id string, trig automation.Trigger, trigStatus change.ID, act automation.Action, v string) automation.Rule {
	return automation.Rule{ID: change.ID(id), Name: "ルール" + id, Trigger: trig, TriggerStatus: trigStatus, Action: act, Value: v, Enabled: true}
}

func records(t *testing.T, s port.Stores, id item.ID) []change.Record {
	t.Helper()
	var recs []change.Record
	_ = s.Read(context.Background(), store, func(r port.Reader) error {
		var err error
		recs, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		return err
	})
	return recs
}

func TestStatusChangeRunsRulesInTheSameSave(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionAssignee, "taro"))
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`)); err != nil {
		t.Fatal(err)
	}
	recs := records(t, rec.Stores, apptest.SampleTask)
	user, auto := recs[len(recs)-2], recs[len(recs)-1]
	if user.F != "status" || auto.F != "assignee" || auto.Op != user.Op || auto.Actor.Rule != "01RULE1" || auto.By != "hanako" {
		t.Errorf("rows = %+v / %+v", user, auto)
	}
	if last(rec) != "HH-2 のステータスを Done に変更" {
		t.Errorf("summary = %q", last(rec))
	}
	ns := inbox.Notices()
	if len(ns) != 1 || ns[0].Kind != port.NoticeAutomation || ns[0].Item != apptest.SampleTask ||
		ns[0].Text != "「ルール01RULE1」で担当者を taro に変更しました" {
		t.Errorf("notices = %+v", ns)
	}
}

func TestAutomationDoesNotChain(t *testing.T) {
	svc, rec, _ := setupAuto(t,
		autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusInProgress, automation.ActionStatus, string(apptest.SampleStatusDone)),
		autoRule("01RULE2", automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionAssignee, "taro"),
	)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusInProgress)+`"`)); err != nil {
		t.Fatal(err)
	}
	it, _ := svc.Get(t.Context(), store, apptest.SampleTask)
	if it.Status != apptest.SampleStatusDone || it.Assignee != "" {
		t.Errorf("item = %+v (RULE2 must not run)", it)
	}
	_ = rec
}

func TestStoppedRuleNotifiesWithTheReason(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusInProgress, automation.ActionStatus, string(apptest.SampleStatusDone)))
	// InProgress からは Pending にだけ行ける
	op := port.Operation{Actor: change.User("taro"), Summary: "制約"}
	_ = rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace,
			change.NewAdd("workflow/"+string(apptest.SampleStatusInProgress), apptest.SampleStatusPending))
	})
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusInProgress)+`"`)); err != nil {
		t.Fatal(err)
	}
	ns := inbox.Notices()
	if len(ns) != 1 || ns[0].Kind != port.NoticeAutomationStopped || ns[0].Text != "「ルール01RULE1」が制約で止まりました" ||
		ns[0].Reason != "ワークフローの制約: InProgress から Done には変更できません(Pending のみ)" || ns[0].Status != apptest.SampleStatusDone {
		t.Errorf("notices = %+v", ns)
	}
	if recs := records(t, rec.Stores, apptest.SampleTask); recs[len(recs)-1].Actor.Kind == change.ActorAutomation {
		t.Error("a stopped run saves nothing")
	}
}

func TestStepsDoneRunsOnTheParentOnce(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStepsDone, "", automation.ActionStatus, string(apptest.SampleStatusDone)))
	a, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "設計")
	b, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "実装")
	if n := len(inbox.Notices()); n != 0 {
		t.Fatalf("AddStep must not run rules: %d", n)
	}
	if _, err := svc.CheckStep(t.Context(), store, a.ID, true); err != nil {
		t.Fatal(err)
	}
	if n := len(inbox.Notices()); n != 0 {
		t.Fatalf("one step is still open: %d", n)
	}
	if _, err := svc.CheckStep(t.Context(), store, b.ID, true); err != nil {
		t.Fatal(err)
	}
	task, _ := svc.Get(t.Context(), store, apptest.SampleTask)
	if task.Status != apptest.SampleStatusDone || len(inbox.Notices()) != 1 {
		t.Errorf("task = %+v, notices = %+v", task, inbox.Notices())
	}
	_ = rec
}

func TestCreateDoesNotRunRules(t *testing.T) {
	svc, _, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusTodo, automation.ActionAssignee, "taro"))
	epic := apptest.SampleEpic
	if _, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: &epic, Kind: item.Task, Title: "新しい",
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(inbox.Notices()); n != 0 {
		t.Errorf("create must not run rules: %d", n)
	}
}

// writeWorkspace は、サンプルのワークスペースのログに行を書く(プロジェクト別のルールの行など)。
func writeWorkspace(t *testing.T, rec *recorder, ds ...change.Draft) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "ワークスペースの設定を変更"}
	err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestStatusChangeRunsScopedRuleOnlyInItsProject(t *testing.T) {
	scoped := func(id string, p project.ID) automation.Rule {
		r := autoRule(id, automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionAssignee, "taro")
		r.Scope = p
		return r
	}
	svc, rec, inbox := setupAuto(t, scoped("01RULE1", otherProject), scoped("01RULE2", apptest.SampleProject))
	addSecondProject(t, rec.Stores)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`)); err != nil {
		t.Fatal(err)
	}
	ns := inbox.Notices()
	if len(ns) != 1 || ns[0].Rule != "01RULE2" {
		t.Errorf("notices = %+v, only the rule of the item's project runs", ns)
	}
	if it, _ := svc.Get(t.Context(), store, apptest.SampleTask); it.Assignee != "taro" {
		t.Errorf("assignee = %q", it.Assignee)
	}
}

func TestStatusChangeSkipsRuleDisabledInProject(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionAssignee, "taro"))
	writeWorkspace(t, rec, change.NewAdd(projectrule.DisabledField(apptest.SampleProject), "01RULE1"))
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`)); err != nil {
		t.Fatal(err)
	}
	if ns := inbox.Notices(); len(ns) != 0 {
		t.Errorf("notices = %+v", ns)
	}
	if it, _ := svc.Get(t.Context(), store, apptest.SampleTask); it.Assignee != "" {
		t.Errorf("assignee = %q, a disabled rule must not run", it.Assignee)
	}
}

func TestStatusChangeStopsOnStatusTheProjectDoesNotUse(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusInProgress, automation.ActionStatus, string(apptest.SampleStatusPending)))
	writeWorkspace(t, rec, change.NewAdd(projectrule.HiddenField(apptest.SampleProject), apptest.SampleStatusPending))
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusInProgress)+`"`)); err != nil {
		t.Fatal(err)
	}
	ns := inbox.Notices()
	if len(ns) != 1 || ns[0].Kind != port.NoticeAutomationStopped || ns[0].Reason != "このプロジェクトでは「Pending」を使いません" || ns[0].Status != apptest.SampleStatusPending {
		t.Errorf("notices = %+v", ns)
	}
	if it, _ := svc.Get(t.Context(), store, apptest.SampleTask); it.Status != apptest.SampleStatusInProgress {
		t.Errorf("status = %s, a stopped run saves nothing", it.Status)
	}
}
