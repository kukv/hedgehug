package githublink_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

const (
	ruleOpen   = "01RULEOPEN0000000000000001"
	ruleMerged = "01RULEMERGED00000000000001"
)

// withRules は、サンプルのワークスペースにルールを書く(B4 の app/items のテストと同じ組み立て)。
func (f *linkFixture) withRules(t *testing.T, rules ...automation.Rule) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "ルールを作成"}
	err := f.stores.Stores.Write(t.Context(), linkStore, op, func(tx port.Tx) error {
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
}

func statusRule(id, name string, trig automation.Trigger, to change.ID) automation.Rule {
	return automation.Rule{ID: change.ID(id), Name: name, Trigger: trig, Action: automation.ActionStatus, Value: string(to), Enabled: true}
}

// taskLog は、HH-2 のログ。
func (f *linkFixture) taskLog(t *testing.T) []change.Record {
	t.Helper()
	var log []change.Record
	err := f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: linkTask})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func (f *linkFixture) taskItem(t *testing.T) item.Item {
	t.Helper()
	var it item.Item
	err := f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		var err error
		it, err = r.Items().Get(linkTask)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return it
}

// ruleRows は、ルールが書いた行。
func ruleRows(log []change.Record) []change.Record {
	var out []change.Record
	for _, r := range log {
		if r.Actor.Rule != "" {
			out = append(out, r)
		}
	}
	return out
}

// putPR は、org/web の番号 n の PR を、状態 s で GitHub に置く。
func (f *linkFixture) putPR(n int, s itemlink.State) {
	pr := pr41
	pr.Number, pr.State = n, s
	f.gh.Put("org/web", pr)
}

func TestAddOpenPRFires(t *testing.T) {
	f := newLinks(t)
	f.withRules(t, statusRule(ruleOpen, "PR が Open で In Progress", automation.TriggerPROpen, apptest.SampleStatusInProgress))
	writes := len(f.stores.ops)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	if n := len(f.stores.ops) - writes; n != 1 {
		t.Errorf("writes = %d, want the rule inside the same save", n)
	}
	if it := f.taskItem(t); it.Status != apptest.SampleStatusInProgress {
		t.Errorf("status = %s", it.Status)
	}
	rows := ruleRows(f.taskLog(t))
	if len(rows) != 1 || rows[0].F != "status" || rows[0].Actor.Rule != ruleOpen || rows[0].By != "hanako" {
		t.Errorf("rule rows = %+v", rows)
	}
	ns := f.inbox.Notices()
	if len(ns) != 1 || ns[0].Kind != port.NoticeAutomation || ns[0].Item != linkTask || ns[0].Rule != ruleOpen ||
		ns[0].Text != "「PR が Open で In Progress」でステータスを InProgress に変更しました" {
		t.Errorf("notices = %+v", ns)
	}
}

func TestAddClosedPRDoesNotFire(t *testing.T) {
	f := newLinks(t)
	f.withRules(t, statusRule(ruleOpen, "PR が Open で In Progress", automation.TriggerPROpen, apptest.SampleStatusInProgress))
	f.putPR(43, itemlink.Closed)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "43", false); err != nil {
		t.Fatal(err)
	}
	if rows := ruleRows(f.taskLog(t)); len(rows) != 0 || len(f.inbox.Notices()) != 0 {
		t.Errorf("rule rows = %+v, notices = %+v", rows, f.inbox.Notices())
	}
}

func TestRefreshMergedFires(t *testing.T) {
	f := newLinks(t)
	f.withRules(t, statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone))
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	f.putPR(41, itemlink.Merged)
	writes := len(f.stores.ops)
	f.refresh(t)
	if n := len(f.stores.ops) - writes; n != 1 {
		t.Errorf("writes = %d, want one save", n)
	}
	if it := f.taskItem(t); it.Status != apptest.SampleStatusDone {
		t.Errorf("status = %s", it.Status)
	}
	rows := ruleRows(f.taskLog(t))
	if len(rows) != 1 || rows[0].Actor.Rule != ruleMerged || rows[0].By != change.BotLogin {
		t.Errorf("rule rows = %+v", rows)
	}
	if ns := f.inbox.Notices(); len(ns) != 1 || ns[0].Text != "「マージで完了」でステータスを Done に変更しました" {
		t.Errorf("notices = %+v", ns)
	}
}

// firstStateOp は、log の中で、取り直しで状態を書いた最初の操作。
func firstStateOp(log []change.Record, since int) change.ID {
	for _, r := range log[since:] {
		if r.Actor.Kind == change.ActorGitHub {
			return r.Op
		}
	}
	return ""
}

func TestRefreshAllMergedOrder(t *testing.T) {
	for _, order := range [][]string{{"41", "42"}, {"42", "41"}} {
		f := newLinks(t)
		r := statusRule(ruleMerged, "すべてマージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone)
		r.AllMerged = true
		f.withRules(t, r)
		f.putPR(42, itemlink.Open)
		for _, n := range order {
			if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", n, false); err != nil {
				t.Fatal(err)
			}
		}
		since := len(f.taskLog(t))
		f.putPR(41, itemlink.Merged)
		f.putPR(42, itemlink.Merged)
		f.refresh(t)
		log := f.taskLog(t)
		rows := ruleRows(log)
		if len(rows) != 1 || len(f.inbox.Notices()) != 1 {
			t.Errorf("order %v: rule rows = %+v, notices = %+v", order, rows, f.inbox.Notices())
			continue
		}
		if first := firstStateOp(log, since); rows[0].Op != first {
			t.Errorf("order %v: the rule ran in op %s, want the first link's save %s", order, rows[0].Op, first)
		}
	}
}

func TestRefreshSameTriggerOnce(t *testing.T) {
	f := newLinks(t)
	f.withRules(t, automation.Rule{ID: ruleMerged, Name: "マージで担当", Trigger: automation.TriggerPRMerged, Action: automation.ActionAssignee, Value: "taro", Enabled: true})
	f.putPR(42, itemlink.Open)
	for _, n := range []string{"41", "42"} {
		if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", n, false); err != nil {
			t.Fatal(err)
		}
	}
	f.putPR(41, itemlink.Merged)
	f.putPR(42, itemlink.Merged)
	f.refresh(t)
	if rows := ruleRows(f.taskLog(t)); len(rows) != 1 || len(f.inbox.Notices()) != 1 {
		t.Errorf("rule rows = %+v, notices = %+v", rows, f.inbox.Notices())
	}
	if it := f.taskItem(t); it.Assignee != "taro" {
		t.Errorf("assignee = %q", it.Assignee)
	}
}

func TestRefreshUnlinkedMeanwhile(t *testing.T) {
	f := newLinks(t)
	f.withRules(t, statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone))
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	f.putPR(41, itemlink.Merged)
	// GitHub から読んだあと、書く前に、誰かがリンクを解除した
	f.stores.before = func() {
		err := f.stores.Stores.Write(t.Context(), linkStore, port.Operation{Actor: change.User("jiro"), Summary: "解除"}, func(tx port.Tx) error {
			log, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: linkTask})
			if err != nil {
				return err
			}
			s := itemlink.FromLog(log)
			return tx.WriteItems().Append(linkTask, s.Detach(s.Links[0])...)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	since := len(f.taskLog(t))
	f.refresh(t)
	for _, r := range f.taskLog(t)[since:] {
		if r.Actor.Kind == change.ActorGitHub || r.Actor.Rule != "" {
			t.Errorf("wrote %+v after the link was removed", r)
		}
	}
	if it := f.taskItem(t); it.Status == apptest.SampleStatusDone || len(f.inbox.Notices()) != 0 {
		t.Errorf("status = %s, notices = %+v", it.Status, f.inbox.Notices())
	}
}

func TestRefreshStoppedNotifies(t *testing.T) {
	f := newLinks(t)
	f.withRules(t, statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone))
	// Todo からは Pending にだけ行ける
	err := f.stores.Stores.Write(t.Context(), linkStore, port.Operation{Actor: change.User("taro"), Summary: "制約"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewAdd("workflow/"+string(apptest.SampleStatusTodo), apptest.SampleStatusPending))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	f.putPR(41, itemlink.Merged)
	f.refresh(t)
	if rows := ruleRows(f.taskLog(t)); len(rows) != 0 {
		t.Errorf("rule rows = %+v, a stopped run writes nothing", rows)
	}
	if it := f.taskItem(t); it.Status != apptest.SampleStatusTodo {
		t.Errorf("status = %s", it.Status)
	}
	ns := f.inbox.Notices()
	if len(ns) != 1 || ns[0].Kind != port.NoticeAutomationStopped || ns[0].Text != "「マージで完了」が制約で止まりました" ||
		ns[0].Reason != "ワークフローの制約: Todo から Done には変更できません(Pending のみ)" || ns[0].Status != apptest.SampleStatusDone {
		t.Errorf("notices = %+v", ns)
	}
}
