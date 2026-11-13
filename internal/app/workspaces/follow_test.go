package workspaces_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ステータス Review を消すと、遷移の制約から外れ、置き換えのアイテムは制約を通さずに変わる(B2-10)。
func TestDeleteStatusFollowsWorkflow(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	statuses := workspace.ParseList("statuses")
	review, _ := s.AddValue(ctx, store, w.ID, statuses, "Review")
	st := settingsOf(t, s, w.ID)
	todo, done := st.Statuses[0].ID, st.Statuses[2].ID
	for _, p := range [][2]change.ID{{todo, review.ID}, {review.ID, done}} {
		if err := s.SetTransition(ctx, store, w.ID, p[0], p[1], true); err != nil {
			t.Fatal(err)
		}
	}
	it := putItem(t, s, w.ID, change.NewSet("status", nil, review.ID))
	if err := s.DeleteValue(ctx, store, w.ID, statuses, review.ID, done); err != nil {
		t.Fatal(err)
	}
	after := settingsOf(t, s, w.ID)
	if _, r := after.Allowed(todo); r || len(after.Workflow[review.ID]) != 0 {
		t.Errorf("workflow = %v", after.Workflow)
	}
	if got := itemOf(t, s, it); got.Status != done {
		t.Errorf("status = %s", got.Status)
	}
}

// 優先度・追加プロパティを消すと、テンプレートの値が置き換わる・外れる。
func TestDeleteValueAndPropertyFollowTemplates(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	env, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "環境", Key: "env", Type: workspace.TypeText})
	high := st.Priorities[1].ID
	tpl, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "調査", Kind: "Task", Values: map[change.ID]json.RawMessage{
		"priority": jsonOf(high), env.ID: jsonOf("本番"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("priorities"), high, st.Priorities[3].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProperty(ctx, store, w.ID, env.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if string(got.Values["priority"]) != string(jsonOf(st.Priorities[3].ID)) || got.Values[env.ID] != nil {
		t.Errorf("values = %v", got.Values)
	}
}

// writeWorkspace は、ワークスペースのログに行を足す(プロジェクト別のルールや専用ルールの準備)。
func writeWorkspace(t *testing.T, s workspaces.Service, ws workspace.ID, ds ...change.Draft) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	if err := s.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(ws, ds...)
	}); err != nil {
		t.Fatal(err)
	}
}

// projectRulesOf は、ワークスペースのログから読んだプロジェクト別のルール。
func projectRulesOf(t *testing.T, s workspaces.Service, ws workspace.ID) projectrule.Set {
	t.Helper()
	d, err := s.Get(t.Context(), store, ws)
	if err != nil {
		t.Fatal(err)
	}
	return d.ProjectRules
}

func newProject(s workspaces.Service, ws workspace.ID) project.Project {
	return project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: ws, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
}

// Review を消すと、プロジェクトの使わないステータスと遷移から外れ、専用ルールは代わりのステータスに
// 書き換わって残る(B5a-16。Review Focus 5)。
func TestDeleteStatusFollowsProjectRules(t *testing.T) {
	s := setup(t)
	s.Inbox = &apptest.Inbox{}
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	statuses := workspace.ParseList("statuses")
	review, _ := s.AddValue(ctx, store, w.ID, statuses, "Review")
	st := settingsOf(t, s, w.ID)
	todo, pending := st.Statuses[0].ID, st.Statuses[3].ID
	p := newProject(s, w.ID)
	putItemIn(t, s, p, 1)
	scoped := automation.Rule{
		ID: "01SCOPED1", Name: "専用", Trigger: automation.TriggerPROpen, Action: automation.ActionStatus,
		Value: string(review.ID), Enabled: true, Scope: p.ID,
	}
	ds := automation.Drafts(automation.Rule{}, scoped)
	ds = append(ds,
		change.NewAdd(projectrule.HiddenField(p.ID), review.ID),
		change.NewAdd(projectrule.WorkflowField(p.ID, todo), review.ID),
		change.NewAdd(projectrule.WorkflowField(p.ID, review.ID), todo),
	)
	writeWorkspace(t, s, w.ID, ds...)
	if err := s.DeleteValue(ctx, store, w.ID, statuses, review.ID, pending); err != nil {
		t.Fatal(err)
	}
	r := projectRulesOf(t, s, w.ID).Of(p.ID)
	if len(r.Hidden) != 0 || len(r.Workflow[todo]) != 0 || len(r.Workflow[review.ID]) != 0 {
		t.Errorf("project rules = %+v", r)
	}
	var rules []automation.Rule
	_ = s.Stores.Read(ctx, store, func(rd port.Reader) error {
		log, err := rd.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		rules = automation.FromLog(log)
		return err
	})
	if len(rules) != 1 || rules[0].Value != string(pending) || rules[0].Scope != p.ID || !rules[0].Enabled {
		t.Errorf("rules = %+v", rules)
	}
}

// テンプレートを消すと、プロジェクトの使うテンプレートと種類ごとの既定から外れる。
func TestDeleteTemplateFollowsProjectRules(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	tpl, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "調査", Kind: "Task"})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "別", Kind: "Task"})
	p := newProject(s, w.ID)
	putItemIn(t, s, p, 1)
	writeWorkspace(t, s, w.ID,
		change.NewAdd(projectrule.TemplatesField(p.ID), tpl.ID),
		change.NewAdd(projectrule.TemplatesField(p.ID), other.ID),
		change.NewSet(projectrule.TemplateDefaultField(p.ID, "Task"), nil, string(tpl.ID)),
	)
	if err := s.DeleteTemplate(ctx, store, w.ID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	r := projectRulesOf(t, s, w.ID).Of(p.ID)
	if len(r.Templates) != 1 || r.Templates[0] != other.ID || r.TemplateDefaults["Task"] != "" {
		t.Errorf("project rules = %+v", r)
	}
}

// テンプレートの種類を変えると、プロジェクトの使うテンプレートと既定から外れる(B5a-16)。
func TestTemplateKindChangeFollowsProjectRules(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	tpl, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "調査", Kind: "Task"})
	if err != nil {
		t.Fatal(err)
	}
	p := newProject(s, w.ID)
	putItemIn(t, s, p, 1)
	writeWorkspace(t, s, w.ID,
		change.NewAdd(projectrule.TemplatesField(p.ID), tpl.ID),
		change.NewSet(projectrule.TemplateDefaultField(p.ID, "Task"), nil, string(tpl.ID)),
	)
	if err := s.UpdateTemplate(ctx, store, w.ID, tpl.ID, workspace.Template{Name: "調査", Kind: "Story"}); err != nil {
		t.Fatal(err)
	}
	r := projectRulesOf(t, s, w.ID).Of(p.ID)
	if len(r.Templates) != 0 || r.TemplateDefaults["Task"] != "" {
		t.Errorf("project rules = %+v", r)
	}
}
