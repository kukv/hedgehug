package repairing_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/repairing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	priorityHigh change.ID = "01SAMPLEVAL000000000000007"
	closed       change.ID = "01CLOSED000000000000000001"
	shipped      change.ID = "01SHIPPED00000000000000001"
	bugTemplate  change.ID = "01TEMPLATE000000000000001"
	notifyRule   change.ID = "01RULE00000000000000000003"
)

var priorities = workspace.ParseList("priorities")

func (e *env) settings() workspace.Settings {
	e.t.Helper()
	var st workspace.Settings
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		var err error
		st, err = r.Workspaces().Settings(apptest.SampleWorkspace)
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return st
}

func (e *env) hasStatus(id change.ID) bool {
	for _, s := range e.settings().Statuses {
		if s.ID == id {
			return true
		}
	}
	return false
}

// workspaceWrite は、ワークスペースのログに ds を書く。
func (e *env) workspaceWrite(ds ...change.Draft) map[change.ID]bool {
	return e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
}

// remote の側で削除した値を、自分の側で進行中の既定にしたら、削除を取り消す。
func TestRestoreReferencedDefault(t *testing.T) {
	e := newEnv(t)
	del := e.workspaceWrite(change.NewDelete(reviewRef))
	pick := e.workspaceWrite(change.NewSet("category_default/doing", apptest.SampleStatusInProgress, review))
	wantOps(t, e.run(pick, del), "「Review」の削除を取り消し")
	if !e.hasStatus(review) {
		t.Errorf("statuses = %+v, want Review", e.settings().Statuses)
	}
}

// 削除の操作で置き換えたアイテムのステータスも戻る。
func TestRestoreReferencedUndoesReplacements(t *testing.T) {
	e := newEnv(t)
	del := e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete(reviewRef)),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", review, apptest.SampleStatusTodo)),
		)
	})
	pick := e.workspaceWrite(change.NewSet("category_default/doing", apptest.SampleStatusInProgress, review))
	wantOps(t, e.run(pick, del), "「Review」の削除を取り消し")
	if it := e.item(apptest.SampleTask); it.Status != review {
		t.Errorf("HH-2 status = %s, want Review", it.Status)
	}
}

// 初期優先度にした値の同時の削除も取り消す。
func TestRestoreReferencedInitialPriority(t *testing.T) {
	e := newEnv(t)
	del := e.workspaceWrite(change.NewDelete(priorities.Ref(priorityHigh)))
	pick := e.workspaceWrite(change.NewSet("initial_priority", apptest.SamplePriorityMedium, priorityHigh))
	wantOps(t, e.run(pick, del), "「High」の削除を取り消し")
	if got := e.settings().InitialPriority; got != priorityHigh {
		t.Errorf("initial priority = %s, want High", got)
	}
}

// 完了の分類が、両側の削除で 0 個になったら、自分の側の削除だけを取り消す。
// 既定の付け替え(remote の側)と同時で、既定は todo の分類にした Closed を指す。
func TestRestoreReferencedEmptyCategory(t *testing.T) {
	e := newEnv(t)
	e.workspaceWrite(append(
		workspace.ValueDrafts(statuses, workspace.Value{ID: closed, Name: "Closed", Color: "gray", Category: workspace.CategoryDone, Rank: "b"}),
		workspace.ValueDrafts(statuses, workspace.Value{ID: shipped, Name: "Shipped", Color: "gray", Category: workspace.CategoryDone, Rank: "c"})...)...)
	theirs := e.workspaceWrite(
		change.NewSet("category_default/done", apptest.SampleStatusDone, closed),
		change.NewDelete(statuses.Ref(apptest.SampleStatusDone)),
	)
	ours := e.workspaceWrite(
		change.NewSet(statuses.Field(closed, "category"), workspace.CategoryDone, workspace.CategoryTodo),
		change.NewDelete(statuses.Ref(shipped)),
	)
	wantOps(t, e.run(ours, theirs), "「Shipped」の削除を取り消し")
	if !e.hasStatus(shipped) || e.hasStatus(apptest.SampleStatusDone) {
		t.Errorf("statuses = %+v, want only Shipped restored", e.settings().Statuses)
	}
}

// 名前の変更と重なった削除を restoreOverlaps が取り消したら、この段は何も書かない。
// restoreReferenced が、restoreOverlaps の取り消した op を飛ばすことの確かめ(二重の取り消しを防ぐ)。
func TestRestoreReferencedAfterOverlap(t *testing.T) {
	e := newEnv(t)
	del := e.workspaceWrite(change.NewDelete(reviewRef))
	ours := e.workspaceWrite(
		change.NewSet(statuses.Field(review, "name"), "Review", "レビュー"),
		change.NewSet("category_default/doing", apptest.SampleStatusInProgress, review),
	)
	wantOps(t, e.run(ours, del), "「レビュー」の削除を取り消し")
}

const (
	webReview   = "プロジェクト「Web」のルールから「Review」を削除(削除された値)"
	webTemplate = "プロジェクト「Web」のルールから「バグ報告」を削除(削除された値)"
	bugDefault  = "Bug の既定のテンプレートを変更(削除された値)"
	notifyOff   = "自動化「レビューで通知」を無効にする(削除された値)"
)

// addWebRules は、サンプルのワークスペースに、プロジェクト「Web」と、その使わないステータス「Review」・
// 使うテンプレート「バグ報告」・Bug の既定「バグ報告」を書く。
func (e *env) addWebRules() {
	web := project.Project{ID: webProject, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 3}, Rank: "X", Title: "Web", Owner: "taro", Status: project.Todo}
	e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(web.ID, project.Create(web)),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace,
				change.NewAdd(projectrule.HiddenField(webProject), review),
				change.NewAdd(projectrule.TemplatesField(webProject), bugTemplate),
				change.NewSet(projectrule.TemplateDefaultField(webProject, "Bug"), nil, string(bugTemplate)),
			),
		)
	})
}

// addTemplate は、テンプレート「バグ報告」を書き、Bug の既定にする。
func (e *env) addTemplate() {
	e.workspaceWrite(
		change.NewSet(workspace.TemplateField(bugTemplate, "name"), nil, "バグ報告"),
		change.NewSet(workspace.TemplateField(bugTemplate, "kind"), nil, "Bug"),
		change.NewSet("template_default/Bug", nil, string(bugTemplate)),
	)
}

// addNotifyRule は、きっかけが「Review」の有効なルール「レビューで通知」を書く。
func (e *env) addNotifyRule(enabled bool) { e.addRuleNamed("レビューで通知", enabled) }

// addRuleNamed は、addNotifyRule の、名前を選べるもの。
func (e *env) addRuleNamed(name string, enabled bool) {
	r := automation.Rule{ID: notifyRule, Name: name, Trigger: automation.TriggerStatus, TriggerStatus: review, Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: enabled}
	e.workspaceWrite(automation.Drafts(automation.Rule{}, r)...)
}

// deleteValues は、「Review」と「バグ報告」を削除する。
func (e *env) deleteValues() {
	e.workspaceWrite(change.NewDelete(reviewRef), change.NewDelete(workspace.TemplateField(bugTemplate)))
}

// 削除された値を指すものは、プロジェクト別のルール(値の ID 順)、種類ごとの既定、ルールの順に直す。
func TestDropDanglingRefs(t *testing.T) {
	e := newEnv(t)
	e.addTemplate()
	e.addWebRules()
	e.addNotifyRule(true)
	e.deleteValues()
	wantOps(t, e.run(nil, nil), webReview, webTemplate, bugDefault, notifyOff)
	set, rules := e.sampleRules()
	if !set.Of(webProject).Empty() {
		t.Errorf("project rules = %+v", set.Of(webProject))
	}
	if st := e.settings(); st.TemplateDefaults["Bug"] != "" {
		t.Errorf("template defaults = %+v", st.TemplateDefaults)
	}
	for _, r := range rules {
		if r.ID == notifyRule && r.Enabled {
			t.Errorf("rule still enabled: %+v", r)
		}
	}
}

// 無効のルールは書かない。
func TestDropDanglingRefsSkipsDisabledRule(t *testing.T) {
	e := newEnv(t)
	e.addNotifyRule(false)
	e.deleteValues()
	wantOps(t, e.run(nil, nil))
}

// 「Web」が同じ Store の別のワークスペースへ移ったら、「メイン」に残ったルールは (9) で外れ、この段は書かない。
func TestDropDanglingRefsAfterOrphans(t *testing.T) {
	e := newEnv(t)
	e.addWebInMobile()
	e.addOrphanRules(webProject)
	e.deleteValues()
	wantOps(t, e.run(nil, nil), orphanWebOp)
}

// ワークフローの辺の両端を別々に削除したら、辺を外す行は先の値の保存だけが持ち、行のなくなった保存は書かない。
func TestDropDanglingRefsEdgeBothEnds(t *testing.T) {
	e := newEnv(t)
	e.addWebRules()
	e.workspaceWrite(append(
		workspace.ValueDrafts(statuses, workspace.Value{ID: shipped, Name: "Shipped", Color: "gray", Category: workspace.CategoryDone, Rank: "c"}),
		change.NewAdd(projectrule.WorkflowField(webProject, review), shipped))...)
	e.workspaceWrite(change.NewDelete(reviewRef))
	e.workspaceWrite(change.NewDelete(statuses.Ref(shipped)))
	wantOps(t, e.run(nil, nil), webReview)
	if set, _ := e.sampleRules(); len(set.Of(webProject).Workflow) != 0 || len(set.Of(webProject).Hidden) != 0 {
		t.Errorf("project rules = %+v", set.Of(webProject))
	}
}

// 完了の分類にされた初期ステータスは、未着手の既定に戻す。「Review」の分類は変えない。
func TestResetFinishedInitial(t *testing.T) {
	e := newEnv(t)
	e.workspaceWrite(
		change.NewSet("initial_status", apptest.SampleStatusTodo, review),
		change.NewSet(statuses.Field(review, "category"), workspace.CategoryDoing, workspace.CategoryDone),
	)
	wantOps(t, e.run(nil, nil), "初期ステータスを「Todo」に戻し")
	st := e.settings()
	if st.InitialStatus != apptest.SampleStatusTodo || !st.Finished(review) {
		t.Errorf("initial = %s, finished(Review) = %v", st.InitialStatus, st.Finished(review))
	}
}

// 1 回の Run で、削除の取り消し、削除された値を指すものの修復、初期ステータスの戻しの順に書く。
func TestValuesStepOrder(t *testing.T) {
	e := newEnv(t)
	e.addTemplate()
	e.addWebRules()
	e.workspaceWrite(change.NewDelete(workspace.TemplateField(bugTemplate)))
	del := e.workspaceWrite(change.NewDelete(reviewRef))
	pick := e.workspaceWrite(
		change.NewSet("category_default/doing", apptest.SampleStatusInProgress, review),
		change.NewSet("initial_status", apptest.SampleStatusTodo, apptest.SampleStatusDone),
	)
	wantOps(t, e.run(pick, del), "「Review」の削除を取り消し", webTemplate, bugDefault, "初期ステータスを「Todo」に戻し")
}

// 使わないステータスを別の側で初期ステータスにしても、修復の保存は書かない。
func TestHiddenInitialNotRepaired(t *testing.T) {
	e := newEnv(t)
	e.addWebRules()
	e.workspaceWrite(change.NewSet("initial_status", apptest.SampleStatusTodo, review))
	wantOps(t, e.run(nil, nil))
}

// 名前の改行と続いた空白は、件名で 1 つの空白にする。
func TestDropDanglingRefsOneLineName(t *testing.T) {
	e := newEnv(t)
	e.addRuleNamed("レビューで\n  通知", true)
	e.deleteValues()
	wantOps(t, e.run(nil, nil), "自動化「レビューで 通知」を無効にする(削除された値)")
}

// noGet は、Projects().Get がどのプロジェクトも読めない Reader。
type noGet struct{ port.Reader }

func (r noGet) Projects() port.ProjectReader { return noGetProjects{r.Reader.Projects()} }

type noGetProjects struct{ port.ProjectReader }

func (noGetProjects) Get(project.ID) (project.Project, error) {
	return project.Project{}, port.ErrNotFound
}

// noGetWriter は、Read に noGet を渡す RepairWriter。
type noGetWriter struct{ *apptest.RepairWriter }

func (w noGetWriter) Read(fn func(r port.Reader) error) error {
	return w.RepairWriter.Read(func(r port.Reader) error { return fn(noGet{r}) })
}

// プロジェクトを読めないときの {タイトル} は、内部 ID。
func TestDropDanglingRefsTitleFallback(t *testing.T) {
	e := newEnv(t)
	e.addTemplate()
	e.addWebRules()
	e.deleteValues()
	w := &apptest.RepairWriter{Stores: e.s, StoreID: store}
	if err := repairing.Run(e.ctx, noGetWriter{w}); err != nil {
		t.Fatal(err)
	}
	id := string(webProject)
	wantOps(t, w.Ops,
		"プロジェクト「"+id+"」のルールから「Review」を削除(削除された値)",
		"プロジェクト「"+id+"」のルールから「バグ報告」を削除(削除された値)",
		bugDefault)
}
