package bulkchange

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// setupAuto は、S1 の Web にルール rules を書き、受信箱を付けた Service。
func setupAuto(t *testing.T, rules ...automation.Rule) (Service, *apptest.Stores, *apptest.Inbox) {
	t.Helper()
	svc, s, _ := setupRun(t, "")
	inbox := &apptest.Inbox{}
	svc.Inbox = inbox
	for _, r := range rules {
		writeWs(t, s, s1, apptest.SampleWorkspace, automation.Drafts(automation.Rule{}, r)...)
	}
	return svc, s, inbox
}

// rule は、ルール「ルール01RULE1」。
func rule(trig automation.Trigger, trigStatus change.ID, act automation.Action, v string) automation.Rule {
	return automation.Rule{ID: "01RULE1", Name: "ルール01RULE1", Trigger: trig, TriggerStatus: trigStatus, Action: act, Value: v, Enabled: true}
}

func itemLog(t *testing.T, s port.Stores, id item.ID) []change.Record {
	t.Helper()
	var out []change.Record
	err := s.Read(t.Context(), s1, func(r port.Reader) (err error) {
		out, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBulkAutomation(t *testing.T) {
	svc, s, inbox := setupAuto(t, rule(automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionAssignee, "taro"))
	if _, err := svc.Run(t.Context(), []Target{{s1, apptest.SampleEpic}, {s1, apptest.SampleTask}}, done); err != nil {
		t.Fatal(err)
	}
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		log := itemLog(t, s, id)
		user, auto := log[len(log)-2], log[len(log)-1]
		if user.F != "status" || auto.F != "assignee" || auto.Op != user.Op || auto.Actor.Rule != "01RULE1" {
			t.Errorf("%s: rows = %+v / %+v", id, user, auto)
		}
		if it := get(t, s, s1, id); it.Assignee != "taro" {
			t.Errorf("%s: assignee = %q", id, it.Assignee)
		}
	}
	ns := inbox.Notices()
	if len(ns) != 2 || ns[0].Kind != port.NoticeAutomation || ns[0].Item != apptest.SampleEpic || ns[1].Item != apptest.SampleTask ||
		ns[0].Text != "「ルール01RULE1」で担当者を taro に変更しました" || ns[1].Display != "HH-2" || ns[0].Store != s1 {
		t.Errorf("notices = %+v", ns)
	}
}

func TestBulkStepsDoneOnce(t *testing.T) {
	svc, s, inbox := setupAuto(t, rule(automation.TriggerStepsDone, "", automation.ActionStatus, string(apptest.SampleStatusDone)))
	story := hh3
	steps := []item.ID{"01SAMPLEIT0000000000000021", "01SAMPLEIT0000000000000022", "01SAMPLEIT0000000000000023"}
	its := []item.Item{work(story, 3, item.Story, "C", "")}
	for i, id := range steps {
		st := work(id, 21+i, item.Step, string(rune('a'+i)), "")
		st.Place.Parent = &story
		its = append(its, st)
	}
	addItems(t, s, its...)
	if _, err := svc.Run(t.Context(), []Target{{s1, steps[0]}, {s1, steps[1]}}, done); err != nil {
		t.Fatal(err)
	}
	if it := get(t, s, s1, story); it.Status != apptest.SampleStatusTodo || len(inbox.Notices()) != 0 {
		t.Fatalf("2 steps: story = %q, notices = %+v", it.Status, inbox.Notices())
	}
	// 残りの 1 件と、すでに Done の 2 件を選ぶ。変えた Step の前の値で判定するので、1 回だけ動く。
	if _, err := svc.Run(t.Context(), []Target{{s1, steps[0]}, {s1, steps[1]}, {s1, steps[2]}}, done); err != nil {
		t.Fatal(err)
	}
	ns := inbox.Notices()
	if it := get(t, s, s1, story); it.Status != apptest.SampleStatusDone || len(ns) != 1 || ns[0].Item != story {
		t.Errorf("story = %q, notices = %+v", it.Status, ns)
	}
}

func TestBulkStepsDoneAllAtOnce(t *testing.T) {
	svc, s, inbox := setupAuto(t, rule(automation.TriggerStepsDone, "", automation.ActionStatus, string(apptest.SampleStatusDone)))
	story := hh3
	steps := []item.ID{"01SAMPLEIT0000000000000021", "01SAMPLEIT0000000000000022", "01SAMPLEIT0000000000000023"}
	its := []item.Item{work(story, 3, item.Story, "C", "")}
	var targets []Target
	for i, id := range steps {
		st := work(id, 21+i, item.Step, string(rune('a'+i)), "")
		st.Place.Parent = &story
		its = append(its, st)
		targets = append(targets, Target{s1, id})
	}
	addItems(t, s, its...)
	if _, err := svc.Run(t.Context(), targets, done); err != nil {
		t.Fatal(err)
	}
	ns := inbox.Notices()
	if it := get(t, s, s1, story); it.Status != apptest.SampleStatusDone || len(ns) != 1 || ns[0].Item != story {
		t.Errorf("story = %q, notices = %+v", it.Status, ns)
	}
}

func TestBulkStepsDoneSkipsDeletedParent(t *testing.T) {
	svc, s, inbox := setupAuto(t, rule(automation.TriggerStepsDone, "", automation.ActionStatus, string(apptest.SampleStatusDone)))
	story := hh3
	step := work("01SAMPLEIT0000000000000021", 21, item.Step, "a", "")
	step.Place.Parent = &story
	addItems(t, s, work(story, 3, item.Story, "C", ""), step)
	if err := apptest.DeleteItems(t.Context(), s, s1, story); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Run(t.Context(), []Target{{s1, step.ID}}, done); err != nil {
		t.Fatal(err)
	}
	if it := get(t, s, s1, story); it.Status != apptest.SampleStatusTodo || len(inbox.Notices()) != 0 {
		t.Errorf("story = %q, notices = %+v", it.Status, inbox.Notices())
	}
}

func TestBulkAutomationStopped(t *testing.T) {
	svc, s, inbox := setupAuto(t, rule(automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionStatus, string(apptest.SampleStatusInProgress)))
	// Done からは Todo にだけ行ける
	writeWs(t, s, s1, apptest.SampleWorkspace, change.NewAdd("workflow/"+string(apptest.SampleStatusDone), apptest.SampleStatusTodo))
	if _, err := svc.Run(t.Context(), []Target{{s1, apptest.SampleTask}}, done); err != nil {
		t.Fatal(err)
	}
	ns := inbox.Notices()
	if len(ns) != 1 || ns[0].Kind != port.NoticeAutomationStopped || ns[0].Text != "「ルール01RULE1」が制約で止まりました" ||
		ns[0].From != apptest.SampleStatusDone || ns[0].Status != apptest.SampleStatusInProgress {
		t.Errorf("notices = %+v", ns)
	}
	if it := get(t, s, s1, apptest.SampleTask); it.Status != apptest.SampleStatusDone {
		t.Errorf("status = %q", it.Status)
	}
	if log := itemLog(t, s, apptest.SampleTask); log[len(log)-1].Actor.Kind == change.ActorAutomation {
		t.Error("a stopped run saves nothing")
	}
}
