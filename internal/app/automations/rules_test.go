package automations_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

// recorder は、保存のアクティビティの文を覚える。
type recorder struct {
	port.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T) (automations.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return automations.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"}, Clock: clock}, rec
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func doneRule() automation.Rule {
	return automation.Rule{
		Name: " 完了で外す ", Trigger: automation.TriggerStatus, TriggerStatus: apptest.SampleStatusDone,
		AllMerged: true, Action: automation.ActionAssignee, Value: automation.Unassigned,
	}
}

func TestCreateUpdateDelete(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "完了で外す" || !r.Enabled || r.AllMerged || r.ID == "" {
		t.Errorf("Create = %+v", r)
	}
	if _, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule()); message(err) != "同じ名前のルールがあります" {
		t.Errorf("duplicate: %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false); err != nil {
		t.Fatal(err)
	}
	edit := doneRule()
	edit.Name = "完了で taro に"
	edit.Value = "taro"
	if err := svc.Update(ctx, store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, store, apptest.SampleWorkspace, r.ID); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"自動化「完了で外す」を作成",
		"自動化「完了で外す」を作成", // 断った保存も Write は呼ばれる(行は残らない)
		"自動化「完了で外す」を無効にする",
		"自動化「完了で taro に」を編集",
		"自動化「完了で taro に」を削除",
	}
	if len(rec.summaries) != len(want) {
		t.Fatalf("summaries = %q", rec.summaries)
	}
	for i := range want {
		if rec.summaries[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, rec.summaries[i], want[i])
		}
	}
}

// 編集では有効・無効を変えない(B4-5)。
func TestUpdateKeepsEnabled(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	_ = svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false)
	edit := doneRule()
	edit.Name = "別名"
	_ = svc.Update(ctx, store, apptest.SampleWorkspace, r.ID, edit)
	v, err := svc.List(ctx, store, apptest.SampleWorkspace)
	if err != nil || len(v.Rules) != 1 || v.Rules[0].Enabled || v.Rules[0].Name != "別名" {
		t.Errorf("List = %+v, %v", v, err)
	}
}

func TestSetEnabledRefusesTheSameStatus(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	_ = svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false)
	// 同期で届いた形を作る: きっかけとすることが同じステータスで無効のルール
	same := automation.Rule{
		ID: "01RULESAME000000000000001", Name: "同じ", Trigger: automation.TriggerStatus,
		TriggerStatus: apptest.SampleStatusDone, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone),
	}
	err := rec.Stores.Write(ctx, store, port.Operation{Actor: change.User("taro"), Summary: "同期"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(automation.Rule{}, same)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, same.ID, true); message(err) != automation.SwitchRefusal {
		t.Errorf("SetEnabled = %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, same.ID, false); err != nil { // OFF は通す
		t.Errorf("SetEnabled(false) = %v", err)
	}
}

func TestBadReferencesAreNotFound(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	bad := doneRule()
	bad.Value = "jiro" // メンバーにいない
	if _, err := svc.Create(ctx, store, apptest.SampleWorkspace, bad); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Create = %v", err)
	}
	if err := svc.Delete(ctx, store, apptest.SampleWorkspace, "01NORULE"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Delete = %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, "01NORULE", true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetEnabled = %v", err)
	}
}

// 担当者の値は、メンバーの保存どおりの綴りで入る(実行は大文字小文字を区別して比べる)。
func TestCreateStoresTheMembersLogin(t *testing.T) {
	svc, _ := setup(t)
	in := doneRule()
	in.Value = "TARO"
	r, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, in)
	if err != nil || r.Value != "taro" {
		t.Fatalf("Create = %+v, %v", r, err)
	}
	v, _ := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if len(v.Rules) != 1 || v.Rules[0].Value != "taro" {
		t.Errorf("List = %+v", v.Rules)
	}
}

// seedScoped は、サンプルのプロジェクトだけのルールを書き、Pending を使わないことにする。
func seedScoped(t *testing.T, rec *recorder) automation.Rule {
	t.Helper()
	r := automation.Rule{
		ID: "01SCOPED0000000000000000001", Name: "専用", Trigger: automation.TriggerPROpen,
		Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone), Enabled: true, Scope: apptest.SampleProject,
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "専用ルールを作成"}
	err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		ds := append(automation.Drafts(automation.Rule{}, r), change.NewAdd(projectrule.HiddenField(apptest.SampleProject), apptest.SampleStatusPending))
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestUpdateRefusesAStatusTheRulesProjectHides(t *testing.T) {
	svc, rec := setup(t)
	r := seedScoped(t, rec)
	edit := r
	edit.Scope = "" // 編集で Scope は変えない。保存済みの Scope で検査する
	edit.Value = string(apptest.SampleStatusPending)
	if err := svc.Update(t.Context(), store, apptest.SampleWorkspace, r.ID, edit); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Update = %v, want ErrNotFound", err)
	}
}

// ステータスの削除の書き換えなどで、することが使わないステータスを指したままの専用ルールも改名できる。
// 変えたきっかけが使わないステータスなら ErrNotFound。
func TestUpdateChecksOnlyChangedStatuses(t *testing.T) {
	svc, rec := setup(t)
	r := seedScoped(t, rec)
	op := port.Operation{Actor: change.User("taro"), Summary: "書き換え"}
	if err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		n := r
		n.Value = string(apptest.SampleStatusPending)
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(r, n)...)
	}); err != nil {
		t.Fatal(err)
	}
	edit := r
	edit.Name, edit.Value = "専用 5", string(apptest.SampleStatusPending)
	if err := svc.Update(t.Context(), store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatalf("rename: %v", err)
	}
	edit.Trigger, edit.TriggerStatus = automation.TriggerStatus, apptest.SampleStatusPending
	if err := svc.Update(t.Context(), store, apptest.SampleWorkspace, r.ID, edit); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("hidden trigger: %v, want ErrNotFound", err)
	}
}

// ワークスペースのルールを消すと、同じ保存で、プロジェクトの「外すルール」からもその ID を除く。
func TestDeleteRemovesTheRuleFromProjectsDisabledRules(t *testing.T) {
	svc, rec := setup(t)
	r, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "外す"}
	if err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewAdd(projectrule.DisabledField(apptest.SampleProject), r.ID))
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(t.Context(), store, apptest.SampleWorkspace, r.ID); err != nil {
		t.Fatal(err)
	}
	err = rec.Read(t.Context(), store, func(rd port.Reader) error {
		log, err := rd.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		if err != nil {
			return err
		}
		if got := projectrule.FromLog(log).Of(apptest.SampleProject).Disabled; len(got) != 0 {
			t.Errorf("Disabled = %v", got)
		}
		last := log[len(log)-1]
		if last.F != projectrule.DisabledField(apptest.SampleProject) || last.Op != log[len(log)-2].Op {
			t.Errorf("not in the same save: %+v", last)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateKeepsTheStoredScope(t *testing.T) {
	svc, rec := setup(t)
	r := seedScoped(t, rec)
	edit := r
	edit.Scope, edit.Name = "", "専用 2"
	if err := svc.Update(t.Context(), store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatal(err)
	}
	var log []change.Record
	err := rec.Read(t.Context(), store, func(rd port.Reader) error {
		var err error
		log, err = rd.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	got := automation.FromLog(log)
	if len(got) != 1 || got[0].Scope != apptest.SampleProject || got[0].Name != "専用 2" {
		t.Errorf("FromLog = %+v", got)
	}
}

// 専用ルールの作成は Scope を保存し、活動の文にプロジェクト名を添える。編集・削除の文も同じ(B5a-10・18)。
func TestCreateScopedRuleSummary(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	in := doneRule()
	in.Scope = apptest.SampleProject
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, in)
	if err != nil || r.Scope != apptest.SampleProject {
		t.Fatalf("Create = %+v, %v", r, err)
	}
	edit := doneRule()
	edit.Name, edit.Value = "専用 2", "taro"
	if err := svc.Update(ctx, store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, store, apptest.SampleWorkspace, r.ID); err != nil {
		t.Fatal(err)
	}
	title := projectTitle(t, rec)
	want := []string{
		"プロジェクト「" + title + "」の自動化「完了で外す」を作成",
		"プロジェクト「" + title + "」の自動化「専用 2」を編集",
		"プロジェクト「" + title + "」の自動化「専用 2」を削除",
	}
	if len(rec.summaries) != len(want) {
		t.Fatalf("summaries = %q", rec.summaries)
	}
	for i := range want {
		if rec.summaries[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, rec.summaries[i], want[i])
		}
	}
}

func projectTitle(t *testing.T, rec *recorder) string {
	t.Helper()
	var title string
	if err := rec.Read(t.Context(), store, func(r port.Reader) error {
		p, err := r.Projects().Get(apptest.SampleProject)
		title = p.Title
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return title
}

// 専用ルールの範囲は、このワークスペースの削除済みでないプロジェクトだけ。ない・削除済みは ErrNotFound。
func TestCreateScopedRuleNeedsALiveProject(t *testing.T) {
	svc, rec := setup(t)
	in := doneRule()
	in.Scope = "01NOPE00000000000000000000"
	if _, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, in); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown project: %v", err)
	}
	if err := apptest.DeleteProject(t.Context(), rec.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	in.Scope = apptest.SampleProject
	if _, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, in); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("deleted project: %v", err)
	}
}

// 別のワークスペースのプロジェクトは範囲にできない。
func TestCreateScopedRuleRefusesAnotherWorkspacesProject(t *testing.T) {
	svc, rec := setup(t)
	other := project.Project{ID: "01OTHERPR0000000000000001", Place: project.Place{Workspace: "01OTHERWS0000000000000001", Number: 1}, Rank: "V", Title: "別", Status: project.Todo}
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	if err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteProjects().Append(other.ID, project.Create(other))
	}); err != nil {
		t.Fatal(err)
	}
	in := doneRule()
	in.Scope = other.ID
	if _, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, in); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("another workspace's project: %v", err)
	}
}

// プロジェクトが削除済みの専用ルールも、編集・削除できる。活動の文はそのプロジェクトの名前を添える。
func TestDeletedProjectsRuleCanBeEditedAndDeleted(t *testing.T) {
	svc, rec := setup(t)
	r := seedScoped(t, rec)
	title := projectTitle(t, rec)
	if err := apptest.DeleteProject(t.Context(), rec.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	edit := r
	edit.Name = "専用 4"
	if err := svc.Update(t.Context(), store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatalf("Update = %v", err)
	}
	if err := svc.Delete(t.Context(), store, apptest.SampleWorkspace, r.ID); err != nil {
		t.Fatalf("Delete = %v", err)
	}
	n := len(rec.summaries)
	want := []string{"プロジェクト「" + title + "」の自動化「専用 4」を編集", "プロジェクト「" + title + "」の自動化「専用 4」を削除"}
	if got := rec.summaries[n-2:]; got[0] != want[0] || got[1] != want[1] {
		t.Errorf("summaries = %q, want %q", got, want)
	}
}

// 編集・削除の活動の文は保存済みの Scope で決める。編集の入力の Scope は見ない。
func TestUpdateKeepsScope(t *testing.T) {
	svc, rec := setup(t)
	r := seedScoped(t, rec)
	edit := r
	edit.Scope, edit.Name = "", "専用 3"
	if err := svc.Update(t.Context(), store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatal(err)
	}
	want := "プロジェクト「" + projectTitle(t, rec) + "」の自動化「専用 3」を編集"
	if got := rec.summaries[len(rec.summaries)-1]; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}
