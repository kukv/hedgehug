package noticing_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	pendingRef  = "statuses/" + string(apptest.SampleStatusPending)
	valueHigh   = change.ID("01SAMPLEVAL000000000000007")
	valueClosed = change.ID("01CLOSED000000000000000001")
	valueShip   = change.ID("01SHIPPED00000000000000001")
)

var (
	statusList   = workspace.ParseList("statuses")
	priorityList = workspace.ParseList("priorities")
)

// nameTaro は、taro の表示名を 太郎 にする。
func nameTaro(f *fixture) {
	f.t.Helper()
	f.save(user("hanako"), func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Rename("taro", "taro", "太郎"))
	})
}

// undoDelete は、op の削除を取り消す、wsRef の修復の保存。
func undoDelete(f *fixture, op change.ID) port.ReceivedSave {
	f.t.Helper()
	return f.repairSave(func(tx port.Tx) error {
		log, err := tx.Changes().Of(wsRef)
		if err != nil {
			return err
		}
		return appendRef(wsRef, change.Undo(log, op)...)(tx)
	})
}

// restoredValue は、setter が prep を書いたあと、hanako が wsRef の fdel を削除し、その削除を取り消す修復の保存を
// 判定して、出た通知を返す。
func restoredValue(f *fixture, setter change.Actor, prep []change.Draft, fdel string) port.Notice {
	f.t.Helper()
	nameTaro(f)
	f.save(setter, appendRef(wsRef, prep...))
	op := f.save(user("hanako"), appendRef(wsRef, change.NewDelete(fdel))).Key.Op
	f.judge(undoDelete(f, op))
	n := f.wantOne()
	if n.Source != "repair:restore:"+string(op) {
		f.t.Errorf("source = %q, want repair:restore:%s", n.Source, op)
	}
	return n
}

func reviewDefault() []change.Draft {
	return []change.Draft{
		change.NewSet(pendingRef+"/name", "Pending", "Review"),
		change.NewSet("category_default/doing", apptest.SampleStatusInProgress, apptest.SampleStatusPending),
	}
}

func TestRestoreTextDefault(t *testing.T) {
	f := setup(t)
	n := restoredValue(f, user("taro"), reviewDefault(), pendingRef)
	if want := "「Review」は 太郎 が同時に既定にしたため、削除を取り消しました"; n.Text != want || n.Section != "statuses" {
		t.Errorf("notice = %+v, want text %q in statuses", n, want)
	}
}

func TestRestoreTextEmptyCategory(t *testing.T) {
	f := setup(t)
	// 完了の分類に Closed と Done(valueShip)を足す。
	closed := workspace.Value{ID: valueClosed, Name: "Closed", Color: "gray", Category: workspace.CategoryDone, Rank: "b"}
	done := workspace.Value{ID: valueShip, Name: "Done", Color: "gray", Category: workspace.CategoryDone, Rank: "c"}
	var prep []change.Draft
	prep = append(prep, workspace.ValueDrafts(statusList, closed)...)
	prep = append(prep, workspace.ValueDrafts(statusList, done)...)
	// 既定を Closed にし、元の Done を削除し、Closed を完了から外すと、完了には valueShip だけが残る。
	prep = append(prep,
		change.NewSet("category_default/done", apptest.SampleStatusDone, valueClosed),
		change.NewDelete(statusList.Ref(apptest.SampleStatusDone)),
		change.NewSet(statusList.Field(valueClosed, "category"), workspace.CategoryDone, workspace.CategoryTodo),
	)
	n := restoredValue(f, user("taro"), prep, statusList.Ref(valueShip))
	if want := "「Done」は完了の最後のステータスのため、削除を取り消しました"; n.Text != want || n.Section != "statuses" {
		t.Errorf("notice = %+v, want text %q in statuses", n, want)
	}
}

func TestRestoreTextInitialPriority(t *testing.T) {
	f := setup(t)
	prep := []change.Draft{change.NewSet("initial_priority", apptest.SamplePriorityMedium, valueHigh)}
	n := restoredValue(f, user("taro"), prep, priorityList.Ref(valueHigh))
	if want := "「High」は 太郎 が同時に既定にしたため、削除を取り消しました"; n.Text != want || n.Section != "priorities" {
		t.Errorf("notice = %+v, want text %q in priorities", n, want)
	}
}

// 名前の変更と重なった取り消し(指されていない値)は、これまでの本文のまま。
func TestRestoreTextOverlapUnchanged(t *testing.T) {
	f := setup(t)
	nameTaro(f)
	f.save(user("taro"), appendRef(wsRef, change.NewSet(pendingRef+"/name", "Pending", "Review")))
	op := f.save(user("hanako"), appendRef(wsRef, change.NewDelete(pendingRef))).Key.Op
	f.save(user("taro"), appendRef(wsRef, change.NewSet(pendingRef+"/name", "Review", "Review2")))
	f.judge(undoDelete(f, op))
	if n, want := f.wantOne(), "「Review2」は 太郎 が同時に変更したため、削除を取り消しました"; n.Text != want {
		t.Errorf("text = %q, want %q", n.Text, want)
	}
}

// 既定にした行が人の行でなければ、変更した人は不明なユーザー。
func TestRestoreTextUnknownSetter(t *testing.T) {
	f := setup(t)
	n := restoredValue(f, change.Actor{Kind: change.ActorSync}, reviewDefault(), pendingRef)
	if want := "「Review」は 不明なユーザー が同時に既定にしたため、削除を取り消しました"; n.Text != want {
		t.Errorf("text = %q, want %q", n.Text, want)
	}
}

const (
	// webProj は、ワークスペース「Web」のプロジェクト「Web」(HH-P3)。
	webProj    project.ID = "01SAMPLEPR000000000000000W"
	notifyRule change.ID  = "01RULE00000000000000000003"
)

// valueRefsSetup は、taro(表示名 太郎)が、プロジェクト「Web」・ステータス「Review」(Pending の名前を変えたもの)・
// Bug のテンプレート「バグ報告」を書いた fixture。
func valueRefsSetup(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	nameTaro(f)
	web := project.Project{ID: webProj, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 3}, Rank: "X", Title: "Web", Owner: "taro", Status: project.Todo}
	f.save(user("taro"), func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(web.ID, project.Create(web)),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace,
				change.NewSet(pendingRef+"/name", "Pending", "Review"),
				change.NewSet(workspace.TemplateField(template, "name"), nil, "バグ報告"),
				change.NewSet(workspace.TemplateField(template, "kind"), nil, "Bug"),
			),
		)
	})
	return f
}

// taroDeletes は、taro が値の項目 fs を 1 回の保存で削除し、その op を返す。
func taroDeletes(f *fixture, fs ...string) change.ID {
	f.t.Helper()
	var ds []change.Draft
	for _, x := range fs {
		ds = append(ds, change.NewDelete(x))
	}
	return f.save(user("taro"), appendRef(wsRef, ds...)).Key.Op
}

// danglingSaves は、いまのログの削除された値を指すものを、修復と同じく 1 つずつ修復の保存にする。
func danglingSaves(f *fixture) []port.ReceivedSave {
	f.t.Helper()
	var out []port.ReceivedSave
	for _, rr := range repair.DanglingRefs(wsLog(f)) {
		out = append(out, wsRepair(f, rr.Drafts...))
	}
	return out
}

// wsLog は、ワークスペース「Web」のいまのログ。
func wsLog(f *fixture) []change.Record {
	f.t.Helper()
	var log []change.Record
	if err := f.stores.Read(f.t.Context(), store, func(r port.Reader) (err error) {
		log, err = r.Changes().Of(wsRef)
		return err
	}); err != nil {
		f.t.Fatal(err)
	}
	return log
}

func TestValueRefsWorkflow(t *testing.T) {
	f := valueRefsSetup(t)
	wsRules(f, "hanako",
		change.NewAdd(projectrule.WorkflowField(webProj, apptest.SampleStatusTodo), apptest.SampleStatusPending),
		change.NewAdd(projectrule.HiddenField(webProj), apptest.SampleStatusPending))
	op := taroDeletes(f, pendingRef)
	f.judge(danglingSaves(f)...)
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Project: webProj, Display: "HH-P3",
		Text:   "プロジェクト「Web」のワークフローの制約から、太郎 が「Review」を削除したため、「Review」を外しました",
		Source: "repair:valueref:" + string(webProj) + ":workflow:" + string(op),
	}
	if n := f.wantOne(); !reflect.DeepEqual(n, want) {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// 使わないステータスだけを外した修復では作らない。
func TestValueRefsHiddenOnly(t *testing.T) {
	f := valueRefsSetup(t)
	wsRules(f, "hanako", change.NewAdd(projectrule.HiddenField(webProj), apptest.SampleStatusPending))
	taroDeletes(f, pendingRef)
	saves := danglingSaves(f)
	if len(saves) != 1 {
		t.Fatalf("saves = %d, want 1", len(saves))
	}
	f.judge(saves...)
	f.wantNone()
}

func TestValueRefsTemplates(t *testing.T) {
	for name, c := range map[string]struct {
		prep    change.Draft
		section string
		text    string
	}{
		"使うテンプレート": {
			change.NewAdd(projectrule.TemplatesField(webProj), template), "templates",
			"プロジェクト「Web」の使うテンプレートから、太郎 が「バグ報告」を削除したため、「バグ報告」を外しました",
		},
		"Bug の既定": {
			change.NewSet(projectrule.TemplateDefaultField(webProj, "Bug"), nil, string(template)), "template_default/Bug",
			"プロジェクト「Web」のBugの既定のテンプレートから、太郎 が「バグ報告」を削除したため、「バグ報告」を外しました",
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := valueRefsSetup(t)
			wsRules(f, "hanako", c.prep)
			op := taroDeletes(f, workspace.TemplateField(template))
			f.judge(danglingSaves(f)...)
			n := f.wantOne()
			if n.Text != c.text || n.Project != webProj || n.Source != "repair:valueref:"+string(webProj)+":"+c.section+":"+string(op) {
				t.Errorf("notice = %+v, want text %q", n, c.text)
			}
		})
	}
}

func TestValueRefsWorkspaceDefault(t *testing.T) {
	f := valueRefsSetup(t)
	wsRules(f, "hanako", change.NewSet("template_default/Bug", nil, string(template)))
	op := taroDeletes(f, workspace.TemplateField(template))
	f.judge(danglingSaves(f)...)
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Section: "templates",
		Text:   "Bugの既定のテンプレートから、太郎 が「バグ報告」を削除したため、「なし」に戻しました",
		Source: "repair:valueref:" + string(apptest.SampleWorkspace) + ":template_default/Bug:" + string(op),
	}
	if n := f.wantOne(); !reflect.DeepEqual(n, want) {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// notifyOnReview は、きっかけが「Review」の有効なルール「レビューで通知」。
func notifyOnReview() automation.Rule {
	return automation.Rule{
		ID: notifyRule, Name: "レビューで通知", Trigger: automation.TriggerStatus, TriggerStatus: apptest.SampleStatusPending,
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true,
	}
}

// ルールを作ったのが taro でも、最後に編集した hanako に知らせる。名前は 1 行にする。
func TestValueRefsRule(t *testing.T) {
	f := valueRefsSetup(t)
	rule := notifyOnReview()
	wsRules(f, "taro", automation.Drafts(automation.Rule{}, rule)...)
	renamed := rule
	renamed.Name = "レビューで\n 通知"
	wsRules(f, "hanako", automation.Drafts(rule, renamed)...)
	op := taroDeletes(f, pendingRef)
	f.judge(danglingSaves(f)...)
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Rule: notifyRule, Section: "automation",
		Text:   "自動化「レビューで 通知」は、太郎 が「Review」を削除したため、無効にしました",
		Source: "repair:valueref:rule:" + string(notifyRule) + ":" + string(op),
	}
	if n := f.wantOne(); !reflect.DeepEqual(n, want) {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// ルールが指すステータスが 2 つとも削除されていたら、値と鍵の op は automation.Statuses の順で最初のもの(きっかけの
// 「Review」)。先に削除した「Done」ではない。
func TestValueRefsRuleFirstStatus(t *testing.T) {
	f := valueRefsSetup(t)
	rule := notifyOnReview()
	rule.Action, rule.Value = automation.ActionStatus, string(apptest.SampleStatusDone)
	wsRules(f, "hanako", automation.Drafts(automation.Rule{}, rule)...)
	taroDeletes(f, statusList.Ref(apptest.SampleStatusDone))
	op := taroDeletes(f, pendingRef)
	f.judge(danglingSaves(f)...)
	n := f.wantOne()
	if want := "自動化「レビューで通知」は、太郎 が「Review」を削除したため、無効にしました"; n.Text != want ||
		n.Source != "repair:valueref:rule:"+string(notifyRule)+":"+string(op) {
		t.Errorf("notice = %+v, want text %q", n, want)
	}
}

// 外したもの・既定・ルールをあなたが入れていなければ作らない。
func TestValueRefsRecipient(t *testing.T) {
	rule := notifyOnReview()
	renamed := rule
	renamed.Name = "レビューで知らせる"
	for name, c := range map[string]struct {
		prep func(f *fixture)
		del  string
	}{
		"ワークフローの制約を足したのが taro": {func(f *fixture) {
			wsRules(f, "taro", change.NewAdd(projectrule.WorkflowField(webProj, apptest.SampleStatusTodo), apptest.SampleStatusPending))
		}, pendingRef},
		"プロジェクトの既定を選んだのが taro": {func(f *fixture) {
			wsRules(f, "taro", change.NewSet(projectrule.TemplateDefaultField(webProj, "Bug"), nil, string(template)))
		}, workspace.TemplateField(template)},
		"ワークスペースの既定にしたのが taro": {func(f *fixture) {
			wsRules(f, "taro", change.NewSet("template_default/Bug", nil, string(template)))
		}, workspace.TemplateField(template)},
		"ルールを最後に編集したのが taro": {func(f *fixture) {
			wsRules(f, "hanako", automation.Drafts(automation.Rule{}, rule)...)
			wsRules(f, "taro", automation.Drafts(rule, renamed)...)
		}, pendingRef},
	} {
		t.Run(name, func(t *testing.T) {
			f := valueRefsSetup(t)
			c.prep(f)
			taroDeletes(f, c.del)
			saves := danglingSaves(f)
			if len(saves) == 0 {
				t.Fatal("no repair saves")
			}
			f.judge(saves...)
			f.wantNone()
		})
	}
}

// 行の持ち主のプロジェクト「アプリ」が同じ Store の別のワークスペースにあれば((9) の形)、(10) の通知を作らない。
func TestValueRefsSkipsOrphanRepair(t *testing.T) {
	f := valueRefsSetup(t)
	if err := apptest.SampleMobile(t.Context(), f.stores, store); err != nil {
		t.Fatal(err)
	}
	wsRules(f, "hanako", change.NewAdd(projectrule.WorkflowField(apptest.SampleMobileProject, apptest.SampleStatusTodo), apptest.SampleStatusPending))
	taroDeletes(f, pendingRef)
	saves := danglingSaves(f)
	if len(saves) != 1 {
		t.Fatalf("saves = %d, want 1", len(saves))
	}
	f.judge(saves...)
	f.wantNone()
}

// 2 台が同じ修復を書いても(行の ID は違う)、受信箱は 1 行。
func TestValueRefsOnceAcrossMachines(t *testing.T) {
	f := valueRefsSetup(t)
	wsRules(f, "hanako", change.NewAdd(projectrule.WorkflowField(webProj, apptest.SampleStatusTodo), apptest.SampleStatusPending))
	taroDeletes(f, pendingRef)
	refs := repair.DanglingRefs(wsLog(f))
	if len(refs) != 1 {
		t.Fatalf("refs = %+v, want one", refs)
	}
	a, b := wsRepair(f, refs[0].Drafts...), wsRepair(f, refs[0].Drafts...)
	f.judge(a, b)
	if rows := f.rows(); len(rows) != 1 {
		t.Errorf("rows = %+v, want one", rows)
	}
}

// ワークフローの行の From と To を 1 回の操作で削除していたら、行は両方の値の保存に入るが、鍵(プロジェクト・区画・
// 削除の op)が同じなので受信箱は 1 行。本文の値は、行が外した To。
func TestValueRefsBothEndsDeleted(t *testing.T) {
	f := valueRefsSetup(t)
	wsRules(f, "hanako", change.NewAdd(projectrule.WorkflowField(webProj, apptest.SampleStatusPending), apptest.SampleStatusDone))
	taroDeletes(f, pendingRef, statusList.Ref(apptest.SampleStatusDone))
	saves := danglingSaves(f)
	if len(saves) != 2 {
		t.Fatalf("saves = %d, want 2", len(saves))
	}
	f.judge(saves...)
	want := "プロジェクト「Web」のワークフローの制約から、太郎 が「Done」を削除したため、「Done」を外しました"
	if rows := f.rows(); len(rows) != 1 || rows[0].Text != want {
		t.Errorf("rows = %+v, want one with %q", rows, want)
	}
}

// From と To を別の操作で削除していたら、値ごとに 1 行。
func TestValueRefsBothEndsSeparateOps(t *testing.T) {
	f := valueRefsSetup(t)
	wsRules(f, "hanako", change.NewAdd(projectrule.WorkflowField(webProj, apptest.SampleStatusPending), apptest.SampleStatusDone))
	taroDeletes(f, pendingRef)
	taroDeletes(f, statusList.Ref(apptest.SampleStatusDone))
	f.judge(danglingSaves(f)...)
	var texts []string
	for _, r := range f.rows() {
		texts = append(texts, r.Text)
	}
	slices.Sort(texts)
	want := []string{
		"プロジェクト「Web」のワークフローの制約から、太郎 が「Done」を削除したため、「Done」を外しました",
		"プロジェクト「Web」のワークフローの制約から、太郎 が「Review」を削除したため、「Review」を外しました",
	}
	if !slices.Equal(texts, want) {
		t.Errorf("texts = %q, want %q", texts, want)
	}
}

func TestFinishedInitialNotice(t *testing.T) {
	for name, c := range map[string]struct {
		setter string
		want   bool
	}{
		"初期ステータスにしたのが hanako": {"hanako", true},
		"初期ステータスにしたのが taro":   {"taro", false},
	} {
		t.Run(name, func(t *testing.T) {
			f := valueRefsSetup(t)
			set := f.save(user(c.setter), appendRef(wsRef, change.NewSet("initial_status", apptest.SampleStatusTodo, apptest.SampleStatusPending)))
			f.save(user("taro"), appendRef(wsRef, change.NewSet(statusList.Field(apptest.SampleStatusPending, "category"), workspace.CategoryTodo, workspace.CategoryDone)))
			d, ok := repair.FinishedInitial(wsLog(f))
			if !ok {
				t.Fatal("no finished initial")
			}
			f.judge(wsRepair(f, d))
			if !c.want {
				f.wantNone()
				return
			}
			want := port.Notice{
				Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Section: "statuses",
				Text:   "「Review」は 太郎 が完了の分類にしたため、初期ステータスを「Todo」に戻しました",
				Source: "repair:initial:" + string(apptest.SampleWorkspace) + ":" + string(set.Entities[0].Rows[0].ID),
			}
			if n := f.wantOne(); !reflect.DeepEqual(n, want) {
				t.Errorf("notice = %+v, want %+v", n, want)
			}
		})
	}
}
