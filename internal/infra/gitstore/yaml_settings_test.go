package gitstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
	"go.yaml.in/yaml/v4"
)

const (
	yWS     workspace.ID = "01YWS000000000000000000001"
	yProj   project.ID   = "01YPR000000000000000000001"
	yItem   item.ID      = "01YIT000000000000000000001"
	yEnv    change.ID    = "01YPROP0000000000000000001"
	yTag    change.ID    = "01YPROP0000000000000000002"
	yEnvVal change.ID    = "01YVAL00000000000000000A01"
	yTagA   change.ID    = "01YVAL00000000000000000B01"
	yTagC   change.ID    = "01YVAL00000000000000000B03"
)

func yIDs() func() change.ID {
	n := 0
	return func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01YVALUE%017d", n))
	}
}

// ySettings は、既定の設定と、それに足す単一選択「環境」・複数選択「タグ」の行。
func ySettings() (workspace.Settings, []change.Draft) {
	s := workspace.DefaultSettings(yIDs())
	env := workspace.Property{ID: yEnv, Key: "env", Name: "環境", Type: workspace.TypeSingle, Kinds: []string{"Story", "Task"}, Rank: "c"}
	tag := workspace.Property{ID: yTag, Key: "tag", Name: "タグ", Type: workspace.TypeMulti, Kinds: []string{"Task"}, Rank: "d"}
	envList := workspace.List{Kind: workspace.ListChoices, Property: yEnv}
	tagList := workspace.List{Kind: workspace.ListChoices, Property: yTag}
	ds := workspace.PropertyDrafts(env)
	ds = append(ds, workspace.ValueDrafts(envList, workspace.Value{ID: yEnvVal, Name: "本番", Color: "gray", Rank: "V"})...)
	ds = append(ds, workspace.PropertyDrafts(tag)...)
	for i, v := range []change.ID{yTagA, "01YVAL00000000000000000B02", yTagC} {
		ds = append(ds, workspace.ValueDrafts(tagList, workspace.Value{ID: v, Name: string(rune('A' + i)), Color: "gray", Rank: string(rune('V' + i))})...)
	}
	return s, ds
}

func ySeed(t *testing.T) (*Stores, port.StoreID, string, workspace.Settings) {
	t.Helper()
	s, id, dir := newStore(t)
	st, extra := ySettings()
	d, err := workspace.CreateWithRank(workspace.Workspace{ID: yWS, Name: "Web", Prefix: "HH"}, st, "V")
	if err != nil {
		t.Fatal(err)
	}
	it := item.Item{ID: yItem, Place: item.Place{Workspace: yWS, Project: yProj, Number: 1}, Kind: item.Task, Rank: "V", Title: "t"}
	pr := project.Project{ID: yProj, Place: project.Place{Workspace: yWS, Number: 1}, Rank: "V", Title: "p", Owner: "taro", Status: project.Todo}
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	err = s.Write(t.Context(), id, op, func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(yWS, append([]change.Draft{d}, extra...)...); err != nil {
			return err
		}
		if err := tx.WriteProjects().Append(yProj, project.Create(pr)); err != nil {
			return err
		}
		return tx.WriteItems().Append(yItem, item.Create(it),
			change.NewSet("status", nil, st.Statuses[0].ID),
			change.NewSet("priority", nil, st.Priorities[2].ID),
			change.NewAdd("systems", st.Systems[0].ID),
			change.NewAdd("systems", st.Systems[1].ID),
			change.NewSet(item.FieldKey(yEnv), nil, yEnvVal),
			change.NewAdd(item.FieldKey(yTag), yTagA),
			change.NewAdd(item.FieldKey(yTag), yTagC),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, id, dir, st
}

func readFileString(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustContain(t *testing.T, name, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s に %q がない:\n%s", name, w, got)
		}
	}
}

func TestWorkspaceYAMLHasSettings(t *testing.T) {
	_, _, dir, st := ySeed(t)
	got := readFileString(t, dir, "workspaces/"+string(yWS)+"/workspace.yaml")
	mustContain(t, "workspace.yaml", got,
		"rank: V\n", "properties:\n", "statuses:\n", "priorities:\n", "systems:\n", "category_defaults:",
		"initial_status: "+string(st.InitialStatus)+" # Todo\n",
		"initial_priority: "+string(st.InitialPriority)+" # Medium\n",
		"key: env\n", "choices:\n", "name: 本番",
		// 選択肢のないプロパティは flow の 1 行。
		"- {id: status, key: status, name: ステータス, type: status, builtin: true, kinds: [Epic, Story, Task, Bug, Step], hidden: false, rank: V}",
	)
	// プロパティの順は設定の順(組み込みのあとに追加)。
	prev := -1
	for _, p := range st.Properties {
		i := strings.Index(got, "id: "+string(p.ID))
		if i <= prev {
			t.Errorf("プロパティ %s の位置 %d が前(%d)より後ろでない:\n%s", p.ID, i, prev, got)
		}
		prev = i
	}
}

func TestItemYAMLHasNameComments(t *testing.T) {
	_, _, dir, st := ySeed(t)
	got := readFileString(t, dir, "items/"+string(yItem)+"/item.yaml")
	mustContain(t, "item.yaml", got,
		"status: "+string(st.Statuses[0].ID)+" # Todo\n",
		"priority: "+string(st.Priorities[2].ID)+" # Medium\n",
		" # "+st.Systems[0].Name+", "+st.Systems[1].Name+"\n",
		" # 環境: 本番\n",
		" # タグ: A, C\n",
	)
}

func TestWorkspaceRenameValueRegeneratesItemYAML(t *testing.T) {
	s, id, dir, st := ySeed(t)
	name := "statuses/" + string(st.Statuses[0].ID) + "/name"
	op := port.Operation{Actor: change.User("taro"), Summary: "値の名前を変える"}
	before := headCount(t, dir)
	err := s.Write(t.Context(), id, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(yWS, change.NewSet(name, "Todo", "未着手"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := headCount(t, dir); n != before+1 {
		t.Errorf("commits = %d, want %d", n, before+1)
	}
	got := readFileString(t, dir, "items/"+string(yItem)+"/item.yaml")
	mustContain(t, "item.yaml", got, "status: "+string(st.Statuses[0].ID)+" # 未着手\n")
	if !worktreeStatus(t, dir).IsClean() {
		t.Error("作業ツリーがきれいでない(item.yaml が同じコミットに入っていない)")
	}
	if log := readFileString(t, dir, "items/"+string(yItem)+"/log.jsonl"); strings.Contains(log, "未着手") {
		t.Errorf("アイテムの log.jsonl に書いている:\n%s", log)
	}
}

func TestRebuildYAMLMatchesWritePath(t *testing.T) {
	_, _, dir, _ := ySeed(t)
	tr := tree{}
	want := map[string]string{}
	for _, rel := range []string{"workspaces/" + string(yWS), "projects/" + string(yProj), "items/" + string(yItem)} {
		tr[rel+"/log.jsonl"] = []byte(readFileString(t, dir, rel+"/log.jsonl"))
	}
	for rel, yml := range map[string]string{
		"workspaces/" + string(yWS): "workspace.yaml", "items/" + string(yItem): "item.yaml",
	} {
		want[rel+"/"+yml] = readFileString(t, dir, rel+"/"+yml)
	}
	if err := rebuildYAML(tr); err != nil {
		t.Fatal(err)
	}
	for p, w := range want {
		if string(tr[p]) != w {
			t.Errorf("%s が書く道と違う:\n--- 同期\n%s\n--- 書く道\n%s", p, tr[p], w)
		}
	}
}

func TestItemYAMLHasPlainValues(t *testing.T) {
	three, half := 3.0, 1.5
	it := item.Item{
		ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x",
		Assignee: "taro", Start: "2026-11-11", End: "2026-11-15", Points: &three, Estimate: &half,
		Fields: map[change.ID]json.RawMessage{
			"01PROP0000000000000000000C": json.RawMessage(`"下書き memo"`),
			"01PROP0000000000000000000D": json.RawMessage(`42`), "01PROP0000000000000000000E": json.RawMessage(`true`),
		},
	}
	meta := change.Meta{Op: "01OP", At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	log := change.Stamp(nil, []change.Draft{item.Create(it)}, meta, yIDs())
	out, err := renderYAML(change.KindItem, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"assignee": "taro", "start": "2026-11-11", "end": "2026-11-15", "points": 3, "estimate": 1.5}
	for k, v := range want {
		if fmt.Sprint(got[k]) != fmt.Sprint(v) {
			t.Errorf("%s = %v, want %v\n%s", k, got[k], v, out)
		}
	}
	f, _ := got["fields"].(map[string]any)
	if f["01PROP0000000000000000000C"] != "下書き memo" || fmt.Sprint(f["01PROP0000000000000000000D"]) != "42" || f["01PROP0000000000000000000E"] != true {
		t.Errorf("fields = %v\n%s", got["fields"], out)
	}
}

func TestWorkspaceYAMLHasWorkflowAndTemplates(t *testing.T) {
	s, id, dir, st := ySeed(t)
	todo, doing := st.Statuses[0].ID, st.Statuses[1].ID
	tpl := workspace.Template{
		ID: "01YTPL00000000000000000001", Name: "リリース作業", Kind: "Epic", Title: "リリース",
		Children: []workspace.TemplateChild{{
			ID: "01YCHD00000000000000000001", Kind: "Story", Title: "QA",
			Steps: []workspace.TemplateStep{{ID: "01YSTP00000000000000000001", Title: "確認", Assignee: workspace.StepParent}},
		}},
	}.Ranked()
	ds := append([]change.Draft{change.NewAdd(workspace.WorkflowField(todo), doing)}, workspace.TemplateDrafts(workspace.Template{}, tpl)...)
	ds = append(ds, change.NewSet("template_default/Epic", nil, tpl.ID))
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(yWS, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
	got := readFileString(t, dir, "workspaces/"+string(yWS)+"/workspace.yaml")
	mustContain(t, "workspace.yaml", got,
		"workflow:\n", "  "+string(todo)+": ["+string(doing)+"] # Todo → InProgress\n",
		"templates:\n", "name: リリース作業\n", "kind: Epic\n", "children:\n", "title: QA",
		"assignee: '@parent'", "template_defaults: {Epic: 01YTPL00000000000000000001}",
	)
}

func TestWorkspaceYAMLHasRules(t *testing.T) {
	s, id, dir, st := ySeed(t)
	path := "workspaces/" + string(yWS) + "/workspace.yaml"
	// ルールがないワークスペースは、テンプレートと同じく空の一覧を 1 行で書く。
	mustContain(t, "workspace.yaml", readFileString(t, dir, path), "rules: []\n")

	done := st.Statuses[2].ID
	r := automation.Rule{
		ID: "01RULE1", Name: "完了で外す", Trigger: automation.TriggerStatus, TriggerStatus: done,
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true,
	}
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(yWS, automation.Drafts(automation.Rule{}, r)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "workspace.yaml", readFileString(t, dir, path),
		"rules:\n", "name: 完了で外す\n", "trigger: status\n",
		"trigger_status: "+string(done)+" # "+st.Statuses[2].Name+"\n",
		"action: assignee\n", "value: '@none'\n", "enabled: true\n",
	)
}
