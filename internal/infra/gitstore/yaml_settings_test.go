package gitstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
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
	out, err := renderYAML(change.KindItem, log, nil, nil)
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

const (
	ySprint  change.ID = "01YSPR00000000000000000001"
	yPlanned change.ID = "01YSPR00000000000000000002"
	yRelease change.ID = "01YREL00000000000000000001"
)

func TestWorkspaceYAMLSprints(t *testing.T) {
	st, _ := ySettings()
	d, err := workspace.CreateWithRank(workspace.Workspace{ID: yWS, Name: "Web", Prefix: "HH"}, st, "V")
	if err != nil {
		t.Fatal(err)
	}
	s := planning.Sprint{ID: ySprint, Name: "Sprint 4", State: planning.Closed, Start: "2026-11-01", End: "2026-11-14", Goal: "ログイン"}
	r := planning.Release{ID: yRelease, Name: "v1.0", State: planning.Unreleased}
	c := planning.Confirmed{Committed: planning.Totals{Points: 5, Count: 2}, Completed: planning.Totals{Points: 3.5, Count: 1}}
	ds := append([]change.Draft{d}, planning.SprintDrafts(planning.Sprint{}, s)...)
	ds = append(ds, planning.CloseDraft(ySprint, c))
	ds = append(ds, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: yPlanned, Name: "Sprint 5", State: planning.Planned})...)
	ds = append(ds, planning.ReleaseDrafts(planning.Release{}, r)...)
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	log := change.Stamp(nil, ds, change.Meta{Op: "01OP1", At: at, Actor: change.User("taro")}, yIDs())
	log = append(log, change.Stamp(log, []change.Draft{planning.DeleteRelease(yRelease)}, change.Meta{Op: "01OP2", At: at, Actor: change.User("taro")}, yIDs())...)
	out, err := renderYAML(change.KindWorkspace, log, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "rules: []\n" +
		"sprints:\n" +
		"  - {id: 01YSPR00000000000000000001, name: Sprint 4, state: closed, start: \"2026-11-01\", end: \"2026-11-14\", goal: ログイン, committed: {points: 5, count: 2}, completed: {points: 3.5, count: 1}, deleted: null}\n" +
		"  - {id: 01YSPR00000000000000000002, name: Sprint 5, state: planned, start: \"\", end: \"\", goal: \"\", committed: null, completed: null, deleted: null}\n" +
		"releases:\n" +
		"  - {id: 01YREL00000000000000000001, name: v1.0, state: unreleased, date: \"\", description: \"\", deleted: {op: 01OP2, at: \"2026-11-13T09:00:00.000Z\", root: true}}\n"
	if !strings.HasSuffix(string(out), want) {
		t.Errorf("workspace.yaml =\n%s\nwant to end with\n%s", out, want)
	}
}

// writeSprint は、ySeed の Store のスプリントを old から sp に変える(ds も同じ保存で書く)。old が空なら作成で、アイテムも入れる。
func writeSprint(t *testing.T, s *Stores, id port.StoreID, old, sp planning.Sprint, ds ...change.Draft) {
	t.Helper()
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "スプリント"}, func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(yWS, append(planning.SprintDrafts(old, sp), ds...)...); err != nil {
			return err
		}
		if old.ID != "" {
			return nil
		}
		return tx.WriteItems().Append(yItem, change.NewSet(item.SprintField, nil, sp.ID))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenameSprintRewritesItemYAML(t *testing.T) {
	s, id, dir, _ := ySeed(t)
	sp := planning.Sprint{ID: ySprint, Name: "Sprint 4", State: planning.Planned}
	writeSprint(t, s, id, planning.Sprint{}, sp)
	itemDir := "items/" + string(yItem)
	mustContain(t, "item.yaml", readFileString(t, dir, itemDir+"/item.yaml"), "sprint: "+string(ySprint)+" # Sprint 4\n")
	log := readFileString(t, dir, itemDir+"/log.jsonl")

	renamed := sp
	renamed.Name = "Sprint 5"
	writeSprint(t, s, id, sp, renamed)
	mustContain(t, "item.yaml", readFileString(t, dir, itemDir+"/item.yaml"), "sprint: "+string(ySprint)+" # Sprint 5\n")
	if got := readFileString(t, dir, itemDir+"/log.jsonl"); got != log {
		t.Errorf("アイテムの log.jsonl が変わった:\n%s", got)
	}
	if !worktreeStatus(t, dir).IsClean() {
		t.Error("作業ツリーがきれいでない(item.yaml が同じコミットに入っていない)")
	}
}

// TestDeleteSprintKeepsItemYAMLFresh は、スプリントの削除だけを書く保存(名前は変わらない)でも、注釈が
// 消えた item.yaml を同じコミットで作り直すこと(同期で作り直したものと同じになる)。
func TestDeleteSprintKeepsItemYAMLFresh(t *testing.T) {
	s, id, dir, _ := ySeed(t)
	sp := planning.Sprint{ID: ySprint, Name: "Sprint 4", State: planning.Planned}
	writeSprint(t, s, id, planning.Sprint{}, sp)
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "削除"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(yWS, planning.DeleteSprint(ySprint))
	})
	if err != nil {
		t.Fatal(err)
	}
	path := "items/" + string(yItem) + "/item.yaml"
	tr := committedTree(t, dir)
	want := string(tr[path])
	if err := rebuildYAML(tr); err != nil {
		t.Fatal(err)
	}
	if string(tr[path]) != want {
		t.Errorf("item.yaml が作り直したものと違う:\n--- 作り直し\n%s\n--- コミット\n%s", tr[path], want)
	}
	mustContain(t, "item.yaml", want, "sprint: "+string(ySprint)+"\n")
}

// TestSprintStateChangeKeepsItemYAML は、スプリントの状態と日付だけを変える保存では、item.yaml を作り直さない
// (注釈の名前は変わらないので、コミットに入らない)こと。
func TestSprintStateChangeKeepsItemYAML(t *testing.T) {
	s, id, dir, _ := ySeed(t)
	sp := planning.Sprint{ID: ySprint, Name: "Sprint 4", State: planning.Planned, Start: "2026-11-01", End: "2026-11-14"}
	writeSprint(t, s, id, planning.Sprint{}, sp)
	itemYAML := filepath.Join(dir, "items", string(yItem), "item.yaml")
	before, err := os.Stat(itemYAML)
	if err != nil {
		t.Fatal(err)
	}
	active := sp
	active.State, active.End = planning.Active, "2026-11-21"
	writeSprint(t, s, id, sp, active)
	// 作り直しても中身は同じでコミットには出ないので、書いたかどうかは更新時刻で見る。
	if after, err := os.Stat(itemYAML); err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("item.yaml を書き直した(err %v)", err)
	}
	stats, err := headCommit(t, dir).Stats()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, st := range stats {
		names = append(names, st.Name)
	}
	if !slices.Contains(names, "workspaces/"+string(yWS)+"/log.jsonl") || slices.Contains(names, "items/"+string(yItem)+"/item.yaml") {
		t.Errorf("コミットのファイル = %v(ワークスペースのログだけで、item.yaml は入らない)", names)
	}
}

// TestRestoreSprintBringsBackComment は、削除したスプリントを復元すると、item.yaml の注釈が戻ること。
func TestRestoreSprintBringsBackComment(t *testing.T) {
	s, id, dir, _ := ySeed(t)
	writeSprint(t, s, id, planning.Sprint{}, planning.Sprint{ID: ySprint, Name: "Sprint 4", State: planning.Planned})
	op := port.Operation{Actor: change.User("taro"), Summary: "削除"}
	err := s.Write(t.Context(), id, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(yWS, planning.DeleteSprint(ySprint))
	})
	if err != nil {
		t.Fatal(err)
	}
	path := "items/" + string(yItem) + "/item.yaml"
	mustContain(t, "item.yaml", readFileString(t, dir, path), "sprint: "+string(ySprint)+"\n")
	err = s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "復元"}, func(tx port.Tx) error {
		log, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: yWS})
		if err != nil {
			return err
		}
		var undo []change.Draft
		for _, r := range log {
			if r.T == change.Delete && r.F == "sprints/"+string(ySprint) {
				undo = change.Undo(log, r.Op)
			}
		}
		if len(undo) == 0 {
			return fmt.Errorf("削除の行がない")
		}
		return tx.WriteWorkspaces().Append(yWS, undo...)
	})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "item.yaml", readFileString(t, dir, path), "sprint: "+string(ySprint)+" # Sprint 4\n")
	if !worktreeStatus(t, dir).IsClean() {
		t.Error("作業ツリーがきれいでない(item.yaml が同じコミットに入っていない)")
	}
}

func TestSyncRenameRewritesItemYAML(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	sp := planning.Sprint{ID: ySprint, Name: "Sprint 1", State: planning.Planned}
	a.write(t, "スプリント", func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(planning.Sprint{}, sp)...); err != nil {
			return err
		}
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet(item.SprintField, nil, ySprint))
	})
	a.sync(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	renamed := sp
	renamed.Name = "Sprint 2"
	a.write(t, "改名", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(sp, renamed)...)
	})
	// b も同じアイテムを変え、受け取るときに合わせる(早送りでなく、合わせた木から作り直す)。
	b.write(t, "タイトル", set("title", "ログイン画面を作る", "ログイン"))
	a.sync(t, remote)
	b.sync(t, remote)
	got := readFileString(t, b.dir, "items/"+string(apptest.SampleTask)+"/item.yaml")
	mustContain(t, "item.yaml", got, "sprint: "+string(ySprint)+" # Sprint 2\n", "title: ログイン\n")
	a.sync(t, remote)
	sameFiles(t, a, b)
}

func TestCloseSprintRowRoundTrip(t *testing.T) {
	s, id, _, _ := ySeed(t)
	sp := planning.Sprint{ID: ySprint, Name: "Sprint 4", State: planning.Active, Start: "2026-11-01", End: "2026-11-14"}
	c := planning.Confirmed{Committed: planning.Totals{Points: 5, Count: 2}, Completed: planning.Totals{Points: 3.5, Count: 1}}
	closed := sp
	closed.State = planning.Closed
	writeSprint(t, s, id, planning.Sprint{}, sp)
	writeSprint(t, s, id, sp, closed, planning.CloseDraft(ySprint, c))
	want := planning.CloseDraft(ySprint, c)
	err := s.Read(t.Context(), id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: yWS})
		if err != nil {
			return err
		}
		found := false
		for _, rec := range log {
			if rec.T == want.T && rec.F == want.F {
				found = string(rec.To) == string(want.To)
			}
		}
		if !found {
			t.Errorf("完了の行 %s %s が読み戻せない", want.T, want.F)
		}
		got := planning.FromLog(log).Sprints
		if len(got) != 1 || got[0].Confirmed == nil || *got[0].Confirmed != c || got[0].State != planning.Closed {
			t.Errorf("sprints = %+v, want confirmed %+v", got, c)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
