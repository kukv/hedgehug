package noticing_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const (
	sprintID = "01SPRINT00000000000000001"
	otherID  = "01OTHER0000000000000000001"
)

// wsCase は、ワークスペースのログの上書きの場面。prep(taro)のあと、hanako が mine を、同時に taro が theirs を書く。
type wsCase struct {
	prep, mine, theirs []change.Draft
	text, section      string
	project            change.ID
}

// set1 は、f を from から to にする 1 行。
func set1(f string, from, to any) []change.Draft { return []change.Draft{change.NewSet(f, from, to)} }

// relist は、f に v を足しておき、hanako が外し、同時に taro が足し直す場面。
func relist(f string, v any, text, section string) wsCase {
	add := []change.Draft{change.NewAdd(f, v)}
	return wsCase{prep: add, mine: []change.Draft{change.NewRemove(f, v)}, theirs: add, text: text, section: section}
}

func TestOverwriteWorkspace(t *testing.T) {
	todo, done := string(apptest.SampleStatusTodo), apptest.SampleStatusDone
	rules := relist("project_rules/"+string(apptest.SampleProject)+"/hidden_statuses", done, "HH-P1 のルールが上書きされました", "")
	rules.project = apptest.SampleProject
	cases := map[string]wsCase{
		"status name": {
			mine: set1("statuses/"+todo+"/name", "Todo", "Ready"), theirs: set1("statuses/"+todo+"/name", "Todo", "Review"),
			text: "Review の名前が上書きされました", section: "statuses",
		},
		"status color": {
			mine: set1("statuses/"+todo+"/color", "gray", "red"), theirs: set1("statuses/"+todo+"/color", "gray", "blue"),
			text: "Todo の色が上書きされました", section: "statuses",
		},
		"status other": {
			mine: set1("statuses/"+todo+"/memo", nil, "a"), theirs: set1("statuses/"+todo+"/memo", nil, "b"),
			text: "Todo の設定が上書きされました", section: "statuses",
		},
		"property": {
			mine: set1("properties/status/name", "ステータス", "状態"), theirs: set1("properties/status/name", "ステータス", "進み具合"),
			text: "進み具合 の名前が上書きされました", section: "properties",
		},
		"name":     {mine: set1("name", "Web", "Web2"), theirs: set1("name", "Web", "Web3"), text: "ワークスペース名が上書きされました", section: "general"},
		"prefix":   {mine: set1("prefix", "HH", "HA"), theirs: set1("prefix", "HH", "HB"), text: "接頭辞が上書きされました", section: "general"},
		"unknown":  {mine: set1("rank", "a", "b"), theirs: set1("rank", "a", "c"), text: "ワークスペースの設定が上書きされました", section: "general"},
		"workflow": relist("workflow/"+todo, done, "ワークフローが上書きされました", "workflow"),
		"initial status": {
			mine: set1("initial_status", todo, string(done)), theirs: set1("initial_status", todo, "x"),
			text: "初期ステータスが上書きされました", section: "statuses",
		},
		"category default": {
			mine: set1("category_default/todo", todo, "a"), theirs: set1("category_default/todo", todo, "b"),
			text: "既定のステータスが上書きされました", section: "statuses",
		},
		"initial priority": {
			mine: set1("initial_priority", nil, "a"), theirs: set1("initial_priority", nil, "b"),
			text: "初期優先度が上書きされました", section: "priorities",
		},
		"template default": {
			mine: set1("template_default/Task", nil, "a"), theirs: set1("template_default/Task", nil, "b"),
			text: "既定のテンプレートが上書きされました", section: "templates",
		},
		"sprint end": {
			prep: set1("sprints/"+sprintID+"/name", nil, "Sprint 25"),
			mine: set1("sprints/"+sprintID+"/end", nil, "2026-11-20"), theirs: set1("sprints/"+sprintID+"/end", nil, "2026-11-21"),
			text: "Sprint 25 の終了日が上書きされました", section: "sprints",
		},
		"release date": {
			prep: set1("releases/"+sprintID+"/name", nil, "v1.0"),
			mine: set1("releases/"+sprintID+"/date", nil, "2026-11-20"), theirs: set1("releases/"+sprintID+"/date", nil, "2026-11-21"),
			text: "v1.0 の日付が上書きされました", section: "releases",
		},
		"rule": {
			mine: set1("rules/"+otherID+"/name", nil, "整理"), theirs: set1("rules/"+otherID+"/name", nil, "朝の整理"),
			text: "自動化「朝の整理」が上書きされました", section: "automation",
		},
		"template": {
			prep: set1("templates/"+otherID+"/name", nil, "バグ報告"),
			mine: set1("templates/"+otherID+"/title", nil, "a"), theirs: set1("templates/"+otherID+"/title", nil, "b"),
			text: "テンプレート「バグ報告」が上書きされました", section: "templates",
		},
		"choice": {
			mine: set1("choices/P1/"+otherID+"/name", nil, "a"), theirs: set1("choices/P1/"+otherID+"/name", nil, "b"),
			text: "b の名前が上書きされました", section: "properties",
		},
		"no name": {
			mine: set1("systems/"+otherID+"/color", nil, "red"), theirs: set1("systems/"+otherID+"/color", nil, "blue"),
			text: otherID + " の色が上書きされました", section: "systems",
		},
		"project rule": rules,
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			if len(c.prep) > 0 {
				f.save(user("taro"), appendRef(wsRef, c.prep...))
			}
			lose(f, wsRef, c.mine, user("taro"), c.theirs)
			n := f.wantOne()
			if n.Kind != port.NoticeOverwrite || n.Text != c.text || n.Section != c.section || n.Project != c.project ||
				n.Workspace != apptest.SampleWorkspace || n.Item != "" {
				t.Errorf("notice = %+v, want text %q section %q project %q", n, c.text, c.section, c.project)
			}
			if c.project != "" && n.Display != "HH-P1" {
				t.Errorf("display = %q", n.Display)
			}
		})
	}
}

// 有効でない行(使えない接頭辞)は、いまの値を決めないので、勝たない。
func TestOverwriteWorkspaceInvalidWinner(t *testing.T) {
	f := setup(t)
	lose(f, wsRef, set1("prefix", "HH", "HA"), user("taro"), set1("prefix", "HH", "bad"))
	f.wantNone()
}

func TestOverwriteItemProperty(t *testing.T) {
	cases := map[string]struct {
		f, text string
	}{
		"builtin":         {"assignee", "担当者が上書きされました"},
		"custom deleted":  {"fields/" + otherID, otherID + "が上書きされました"},
		"sprint":          {"sprint", "スプリントが上書きされました"},
		"kind":            {"kind", "種類が上書きされました"},
		"unknown is none": {"memo", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			lose(f, epicRef, set1(c.f, nil, "a"), user("taro"), set1(c.f, nil, "b"))
			if c.text == "" {
				f.wantNone()
				return
			}
			if n := f.wantOne(); n.Text != c.text || n.Field != c.f {
				t.Errorf("notice = %+v, want %q", n, c.text)
			}
		})
	}
}
