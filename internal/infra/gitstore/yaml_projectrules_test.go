package gitstore

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	yTpl  change.ID = "01YTPL00000000000000000001"
	yRule change.ID = "01YRULE0000000000000000001"
)

func TestWorkspaceYAMLHasProjectRules(t *testing.T) {
	s, id, dir, st := ySeed(t)
	path := "workspaces/" + string(yWS) + "/workspace.yaml"
	// 専用ルールもプロジェクト別のルールもなければ、節も scope も書かない。
	got := readFileString(t, dir, path)
	for _, no := range []string{"project_rules", "scope"} {
		if strings.Contains(got, no) {
			t.Fatalf("%s がある:\n%s", no, got)
		}
	}

	todo, doing, done := st.Statuses[0].ID, st.Statuses[1].ID, st.Statuses[2].ID
	tpl := workspace.Template{ID: yTpl, Name: "リリース作業", Kind: "Task", Title: "リリース"}.Ranked()
	own := automation.Rule{
		ID: yRule, Name: "完了で外す", Trigger: automation.TriggerStatus, TriggerStatus: done,
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true, Scope: yProj,
	}
	ds := workspace.TemplateDrafts(workspace.Template{}, tpl)
	ds = append(ds, automation.Drafts(automation.Rule{}, own)...)
	ds = append(ds,
		change.NewAdd(projectrule.HiddenField(yProj), doing),
		change.NewAdd(projectrule.HiddenField(yProj), done),
		change.NewAdd(projectrule.WorkflowField(yProj, todo), done),
		change.NewAdd(projectrule.TemplatesField(yProj), yTpl),
		change.NewSet(projectrule.TemplateDefaultField(yProj, "Task"), nil, yTpl),
		change.NewSet(projectrule.TemplateDefaultField(yProj, "Bug"), nil, projectrule.None),
		change.NewAdd(projectrule.DisabledField(yProj), yRule),
	)
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(yWS, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
	got = readFileString(t, dir, path)
	mustContain(t, "workspace.yaml", got,
		"scope: "+string(yProj)+" # HH-P1\n",
		"project_rules:\n",
		"  "+string(yProj)+": # HH-P1\n",
		"    hidden_statuses: ["+string(doing)+", "+string(done)+"] # InProgress・Done\n",
		"    workflow:\n",
		"      "+string(todo)+": ["+string(done)+"] # Todo → Done\n",
		"    templates: ["+string(yTpl)+"] # リリース作業\n",
		"    template_defaults:\n",
		"      Task: "+string(yTpl)+" # リリース作業\n",
		"      Bug: '@none' # なし\n",
		"    disabled_rules: ["+string(yRule)+"] # 完了で外す\n",
	)

	// 同期のあとの作り直しも、書く道と同じ注釈になる。
	tr := tree{}
	for _, rel := range []string{"workspaces/" + string(yWS), "projects/" + string(yProj)} {
		tr[rel+"/log.jsonl"] = []byte(readFileString(t, dir, rel+"/log.jsonl"))
	}
	if err := rebuildYAML(tr); err != nil {
		t.Fatal(err)
	}
	if string(tr["workspaces/"+string(yWS)+"/workspace.yaml"]) != got {
		t.Errorf("同期の作り直しが書く道と違う:\n%s", tr["workspaces/"+string(yWS)+"/workspace.yaml"])
	}
}
