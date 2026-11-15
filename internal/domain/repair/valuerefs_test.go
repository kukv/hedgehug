package repair_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// reviewRule は、きっかけが「Review」のルール R1。
func reviewRule(enabled bool) automation.Rule {
	return automation.Rule{
		ID: "R1", Name: "レビュー", Trigger: automation.TriggerStatus, TriggerStatus: "s-review",
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: enabled,
	}
}

// refLog は、ステータス Todo・Review・Done、テンプレート「バグ報告」(t-bug。Bug の既定)、
// 「Web」(P1)のルール(使わない Review、ワークフロー Todo→Review・Review→Done、使うテンプレート t-bug、Bug の既定 t-bug)、
// ルール R1 を持つログ。
func refLog(enabled bool) []change.Record {
	var ds []change.Draft
	ds = append(ds, status("s-todo", "Todo", workspace.CategoryTodo)...)
	ds = append(ds, status("s-review", "Review", workspace.CategoryDoing)...)
	ds = append(ds, status("s-done", "Done", workspace.CategoryDone)...)
	ds = append(ds, change.NewSet("initial_status", nil, "s-todo"), change.NewSet("category_default/todo", nil, "s-todo"),
		change.NewSet(workspace.TemplateField("t-bug", "name"), nil, "バグ報告"),
		change.NewSet(workspace.TemplateField("t-bug", "kind"), nil, "Bug"),
		change.NewSet("template_default/Bug", nil, "t-bug"))
	log := put(wsBase(), "v0", day, "taro", ds...)
	log = put(log, "p1", day, "taro",
		change.NewAdd(projectrule.HiddenField("P1"), "s-review"),
		change.NewAdd(projectrule.WorkflowField("P1", "s-todo"), "s-review"),
		change.NewAdd(projectrule.WorkflowField("P1", "s-review"), "s-done"),
		change.NewAdd(projectrule.TemplatesField("P1"), "t-bug"),
		change.NewSet(projectrule.TemplateDefaultField("P1", "Bug"), nil, "t-bug"),
	)
	return put(log, "r1", day, "taro", automation.Drafts(automation.Rule{}, reviewRule(enabled))...)
}

// deleteRefs は、log に「Review」(d1)と「バグ報告」(d2)の削除を足す。
func deleteRefs(log []change.Record) []change.Record {
	log = put(log, "d1", day, "hanako", change.NewDelete("statuses/s-review"))
	return put(log, "d2", day, "hanako", change.NewDelete(workspace.TemplateField("t-bug")))
}

func TestDanglingRefs(t *testing.T) {
	got := repair.DanglingRefs(deleteRefs(refLog(true)))
	want := []repair.RefRepair{
		{Kind: repair.RefProject, Project: "P1", Value: "s-review", Drafts: []change.Draft{
			change.NewRemove(projectrule.HiddenField("P1"), change.ID("s-review")),
			change.NewRemove(projectrule.WorkflowField("P1", "s-review"), change.ID("s-done")),
			change.NewRemove(projectrule.WorkflowField("P1", "s-todo"), change.ID("s-review")),
		}},
		{Kind: repair.RefProject, Project: "P1", Value: "t-bug", Drafts: []change.Draft{
			change.NewRemove(projectrule.TemplatesField("P1"), change.ID("t-bug")),
			change.NewSet(projectrule.TemplateDefaultField("P1", "Bug"), "t-bug", nil),
		}},
		{Kind: repair.RefTemplateDefault, Value: "t-bug", TemplateKind: "Bug", Drafts: []change.Draft{
			change.NewSet("template_default/Bug", "t-bug", nil),
		}},
		{Kind: repair.RefRule, Value: "s-review", Rule: "R1", Drafts: []change.Draft{
			change.NewSet(automation.Field("R1", "enabled"), true, false),
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestDanglingRefsSkipsDisabledRule(t *testing.T) {
	var kinds []repair.RefKind
	for _, r := range repair.DanglingRefs(deleteRefs(refLog(false))) {
		kinds = append(kinds, r.Kind)
	}
	want := []repair.RefKind{repair.RefProject, repair.RefProject, repair.RefTemplateDefault}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("無効のルールだけが入らず、ほかは残る: got %v, want %v", kinds, want)
	}
}

func TestDanglingRefsTemplateLimitLifted(t *testing.T) {
	log := deleteRefs(refLog(true))
	for _, r := range repair.DanglingRefs(log) {
		log = put(log, "x"+string(r.Value)+string(r.Rule)+r.TemplateKind, day, "sync", r.Drafts...)
	}
	if got := projectrule.FromLog(log)["P1"]; len(got.Templates) != 0 || !got.Empty() {
		t.Fatalf("限定が空になり、ルールも残らない: %+v", got)
	}
	if got := repair.DanglingRefs(log); len(got) != 0 {
		t.Fatalf("直したあとは何も残らない: %+v", got)
	}
}

func TestDanglingRefsRestored(t *testing.T) {
	log := put(deleteRefs(refLog(true)), "u1", day, "hanako",
		change.NewRestore("statuses/s-review", "d1"), change.NewRestore(workspace.TemplateField("t-bug"), "d2"))
	if got := repair.DanglingRefs(log); len(got) != 0 {
		t.Fatalf("取り消し済みの削除は数えない: %+v", got)
	}
}

func TestDanglingRefsHiddenInitial(t *testing.T) {
	log := put(refLog(true), "a1", day, "taro", status("s-old", "Old", workspace.CategoryTodo)...)
	log = put(log, "i1", day, "hanako", change.NewSet("initial_status", "s-todo", "s-review"))
	log = put(log, "d1", day, "jiro", change.NewDelete("statuses/s-old"))
	if got := repair.DanglingRefs(log); len(got) != 0 {
		t.Fatalf("使わないステータスが初期ステータスでも直さない: %+v", got)
	}
}

// ワークフローの辺の両端を別々に削除したら、辺を外す行は、値の ID の順で先の修復だけが持つ。
func TestDanglingRefsEdgeBothEnds(t *testing.T) {
	log := put(refLog(false), "d1", day, "hanako", change.NewDelete("statuses/s-review"))
	log = put(log, "d3", day, "jiro", change.NewDelete("statuses/s-todo"))
	want := []repair.RefRepair{
		{Kind: repair.RefProject, Project: "P1", Value: "s-review", Drafts: []change.Draft{
			change.NewRemove(projectrule.HiddenField("P1"), change.ID("s-review")),
			change.NewRemove(projectrule.WorkflowField("P1", "s-review"), change.ID("s-done")),
			change.NewRemove(projectrule.WorkflowField("P1", "s-todo"), change.ID("s-review")),
		}},
	}
	if got := repair.DanglingRefs(log); !reflect.DeepEqual(got, want) {
		t.Fatalf("同じ行は 1 回だけ、行のなくなった修復は入れない: got %+v\nwant %+v", got, want)
	}
}

// 先の修復が持つ行は、あとの修復から除き、ほかの行は残す。
func TestDanglingRefsEdgeBothEndsKeepsOthers(t *testing.T) {
	log := put(refLog(false), "d1", day, "hanako", change.NewDelete("statuses/s-review"))
	log = put(log, "d3", day, "jiro", change.NewDelete("statuses/s-done"))
	want := []repair.RefRepair{
		{Kind: repair.RefProject, Project: "P1", Value: "s-done", Drafts: []change.Draft{
			change.NewRemove(projectrule.WorkflowField("P1", "s-review"), change.ID("s-done")),
		}},
		{Kind: repair.RefProject, Project: "P1", Value: "s-review", Drafts: []change.Draft{
			change.NewRemove(projectrule.HiddenField("P1"), change.ID("s-review")),
			change.NewRemove(projectrule.WorkflowField("P1", "s-todo"), change.ID("s-review")),
		}},
	}
	if got := repair.DanglingRefs(log); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
