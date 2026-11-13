package projectrules_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/projectrules"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	store port.StoreID = "S1"
	tpl   change.ID    = "01TESTTPL00000000000000001"
	wsRul change.ID    = "01TESTRUL00000000000000001"
	own1  change.ID    = "01TESTRUL00000000000000002"
	own2  change.ID    = "01TESTRUL00000000000000003"
)

var p = apptest.SampleProject

// recorder は、保存のアクティビティの文を覚える。
type recorder struct {
	port.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

// setup は、サンプルに、テンプレート 1 つと、ワークスペース全体のルール 1 つ・プロジェクト専用のルール 2 つを足す。
func setup(t *testing.T) (projectrules.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore(store)
	ctx := context.Background()
	if err := apptest.Sample(ctx, s, store); err != nil {
		t.Fatal(err)
	}
	rule := func(id change.ID, name string, scope string, st change.ID) []change.Draft {
		return automation.Drafts(automation.Rule{}, automation.Rule{
			ID: id, Name: name, Scope: change.ID(scope), Trigger: automation.TriggerStatus, TriggerStatus: st,
			Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true,
		})
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	err := s.Write(ctx, store, op, func(tx port.Tx) error {
		ds := workspace.TemplateDrafts(workspace.Template{}, workspace.Template{ID: tpl, Name: "調査", Kind: "Task"})
		ds = append(ds, rule(wsRul, "完了で外す", "", apptest.SampleStatusDone)...)
		ds = append(ds, rule(own1, "保留で外す", string(p), apptest.SampleStatusPending)...)
		ds = append(ds, rule(own2, "取消で外す", string(p), apptest.SampleStatusCancel)...)
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return projectrules.Service{Stores: rec, Identity: apptest.Identity{Login: "hanako"}}, rec
}

func rulesOf(t *testing.T, svc projectrules.Service) projectrule.Rules {
	t.Helper()
	var set projectrule.Set
	err := svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		set = projectrule.FromLog(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return set.Of(p)
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func TestOperationsSaveRowsAndActivity(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	// 専用ルールが Pending を使うので、先に専用ルールを戻しておく
	if err := svc.Reset(ctx, store, p, projectrule.SectionAutomation); err != nil {
		t.Fatal(err)
	}
	rec.summaries = nil
	steps := []struct {
		name string
		do   func() error
		want string
	}{
		{"status", func() error { return svc.SetStatusUsed(ctx, store, p, apptest.SampleStatusPending, false) }, "プロジェクト「ログイン」の使うステータスを変更"},
		{"workflow", func() error {
			return svc.SetTransition(ctx, store, p, apptest.SampleStatusTodo, apptest.SampleStatusDone, true)
		}, "プロジェクト「ログイン」のワークフローの制約を変更"},
		{"template", func() error { return svc.SetTemplateUsed(ctx, store, p, tpl, true) }, "プロジェクト「ログイン」の使うテンプレートを変更"},
		{"default", func() error { return svc.SetTemplateDefault(ctx, store, p, "Task", string(tpl)) }, "プロジェクト「ログイン」のTaskの既定のテンプレートを変更"},
		{"runs off", func() error { return svc.SetRuleRuns(ctx, store, p, wsRul, false) }, "プロジェクト「ログイン」で自動化「完了で外す」を動かさないようにする"},
		{"runs on", func() error { return svc.SetRuleRuns(ctx, store, p, wsRul, true) }, "プロジェクト「ログイン」で自動化「完了で外す」を動かすようにする"},
	}
	for _, c := range steps {
		if err := c.do(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := rec.summaries[len(rec.summaries)-1]; got != c.want {
			t.Errorf("%s: %q", c.name, got)
		}
	}
	got := rulesOf(t, svc)
	want := projectrule.Rules{
		Hidden:           []change.ID{apptest.SampleStatusPending},
		Workflow:         map[change.ID][]change.ID{apptest.SampleStatusTodo: {apptest.SampleStatusDone}},
		Templates:        []change.ID{tpl},
		TemplateDefaults: map[string]string{"Task": string(tpl)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %+v", got)
	}
}

func TestNoOpDoesNotSave(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := svc.SetStatusUsed(ctx, store, p, apptest.SampleStatusDone, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetRuleRuns(ctx, store, p, wsRul, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetTemplateDefault(ctx, store, p, "Task", ""); err != nil {
		t.Fatal(err)
	}
	if len(rec.summaries) != 0 {
		t.Errorf("保存された: %v", rec.summaries)
	}
}

func TestStatusUsedRefusesOwnRule(t *testing.T) {
	svc, rec := setup(t)
	err := svc.SetStatusUsed(t.Context(), store, p, apptest.SampleStatusPending, false)
	want := "「Pending」はこのプロジェクトだけのルール「保留で外す」が使っているため、使わないことにできません。先にルールを変えてください"
	if message(err) != want {
		t.Fatalf("%v", err)
	}
	if len(rec.summaries) != 0 || len(rulesOf(t, svc).Hidden) != 0 {
		t.Error("断りなのに保存された")
	}
}

func TestUnknownIsNotFound(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	for name, err := range map[string]error{
		"status":   svc.SetStatusUsed(ctx, store, p, "nope", false),
		"template": svc.SetTemplateUsed(ctx, store, p, "nope", true),
		"kind":     svc.SetTemplateDefault(ctx, store, p, "Nope", ""),
		"scoped":   svc.SetRuleRuns(ctx, store, p, own1, false),
		"norule":   svc.SetRuleRuns(ctx, store, p, "nope", false),
		"project":  svc.SetStatusUsed(ctx, store, "nope", apptest.SampleStatusPending, true),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestDeletedProjectIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteProject(ctx, svc.Stores, store, p); err != nil {
		t.Fatal(err)
	}
	rec.summaries = nil
	for name, err := range map[string]error{
		"status":   svc.SetStatusUsed(ctx, store, p, apptest.SampleStatusDone, true),
		"workflow": svc.SetTransition(ctx, store, p, apptest.SampleStatusTodo, apptest.SampleStatusDone, true),
		"template": svc.SetTemplateUsed(ctx, store, p, tpl, true),
		"default":  svc.SetTemplateDefault(ctx, store, p, "Task", string(tpl)),
		"runs":     svc.SetRuleRuns(ctx, store, p, wsRul, false),
		"reset":    svc.Reset(ctx, store, p, projectrule.SectionAutomation),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(rec.summaries) != 0 {
		t.Errorf("保存された: %v", rec.summaries)
	}
}

func TestWorkflowRefusedByWorkspace(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	op := port.Operation{Actor: change.User("taro"), Summary: "制約"}
	err := svc.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace,
			change.NewAdd("workflow/"+string(apptest.SampleStatusTodo), apptest.SampleStatusInProgress))
	})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.SetTransition(ctx, store, p, apptest.SampleStatusTodo, apptest.SampleStatusDone, true)
	if message(err) != "ワークスペースの制約で変更できません" {
		t.Errorf("%v", err)
	}
}

func TestResetAutomationDeletesScopedRules(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := svc.SetRuleRuns(ctx, store, p, wsRul, false); err != nil {
		t.Fatal(err)
	}
	rec.summaries = nil
	if err := svc.Reset(ctx, store, p, projectrule.SectionAutomation); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rec.summaries, []string{"プロジェクト「ログイン」の自動化をワークスペースの設定に戻す"}) {
		t.Errorf("summaries = %v", rec.summaries)
	}
	if len(rulesOf(t, svc).Disabled) != 0 {
		t.Error("disabled_rules が残った")
	}
	var left []automation.Rule
	err := svc.Stores.Read(ctx, store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		left = automation.FromLog(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].ID != wsRul {
		t.Errorf("rules = %+v", left)
	}
}

func TestResetSections(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := svc.SetTemplateUsed(ctx, store, p, tpl, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetTemplateDefault(ctx, store, p, "Task", string(tpl)); err != nil {
		t.Fatal(err)
	}
	rec.summaries = nil
	if err := svc.Reset(ctx, store, p, projectrule.SectionTemplates); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rec.summaries, []string{"プロジェクト「ログイン」のテンプレートをワークスペースの設定に戻す"}) {
		t.Errorf("summaries = %v", rec.summaries)
	}
	if got := rulesOf(t, svc); len(got.Templates) != 0 || len(got.TemplateDefaults) != 0 {
		t.Errorf("rules = %+v", got)
	}
}

func TestResetWithoutRulesIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	for _, sec := range []projectrule.Section{projectrule.SectionStatuses, projectrule.SectionWorkflow, projectrule.SectionTemplates} {
		if err := svc.Reset(t.Context(), store, p, sec); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", sec, err)
		}
	}
	if len(rec.summaries) != 0 {
		t.Errorf("保存された: %v", rec.summaries)
	}
}
