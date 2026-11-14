package bulkchange

import (
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// jctx は、サンプルの設定(workspace.DefaultSettings)から、種類 kind・ステータス Todo のアイテムの判定の材料を組む。
func jctx(kind item.Kind) itemContext {
	n := 0
	st := workspace.DefaultSettings(func() change.ID {
		n++
		return change.ID(fmt.Sprintf("V%02d", n))
	})
	return itemContext{
		it:       item.Item{ID: "I1", Kind: kind, Status: valueID(st.Statuses, "Todo"), Priority: valueID(st.Priorities, "Medium")},
		settings: st,
		members:  []member.Member{{Login: "hanako", Name: "花子"}, {Login: "taro", Name: "太郎"}},
		plan: planning.Plan{
			Sprints: []planning.Sprint{
				{ID: "SP1", Name: "S-1", State: planning.Planned},
				{ID: "SP2", Name: "S-2", State: planning.Closed},
				{ID: "SP3", Name: "S-3", State: planning.Planned, Deleted: &change.Deletion{}},
			},
			Releases: []planning.Release{
				{ID: "R1", Name: "R-1"},
				{ID: "R2", Name: "R-2", State: planning.Archived},
			},
		},
	}
}

func valueID(vs []workspace.Value, name string) change.ID {
	for _, v := range vs {
		if v.Name == name {
			return v.ID
		}
	}
	panic("no value " + name)
}

// editProperty は、組み込みのプロパティ key の設定を f で書き換える。
func editProperty(c itemContext, key string, f func(*workspace.Property)) itemContext {
	c.settings.Properties = slices.Clone(c.settings.Properties)
	for i := range c.settings.Properties {
		if c.settings.Properties[i].Key == key {
			f(&c.settings.Properties[i])
		}
	}
	return c
}

func dropKind(c itemContext, key string, kind item.Kind) itemContext {
	return editProperty(c, key, func(p *workspace.Property) {
		p.Kinds = slices.DeleteFunc(slices.Clone(p.Kinds), func(k string) bool { return k == string(kind) })
	})
}

func wantReason(t *testing.T, got verdict, reason string) {
	t.Helper()
	if got.reason != reason || len(got.drafts) != 0 {
		t.Fatalf("verdict = %+v, want reason %q", got, reason)
	}
}

func wantSame(t *testing.T, got verdict) {
	t.Helper()
	if got.reason != "" || len(got.drafts) != 0 || got.sprint != nil {
		t.Fatalf("verdict = %+v, want same", got)
	}
}

func wantDrafts(t *testing.T, got verdict, want ...change.Draft) {
	t.Helper()
	same := func(a, b change.Draft) bool {
		return a.T == b.T && a.F == b.F && string(a.From) == string(b.From) && string(a.To) == string(b.To)
	}
	if got.reason != "" || !slices.EqualFunc(got.drafts, want, same) {
		t.Fatalf("verdict = %+v, want drafts %+v", got, want)
	}
}

func TestJudgeOrder(t *testing.T) {
	c := jctx(item.Epic)
	c.readOnly, c.deleted = true, true
	wantReason(t, judge(c, parsed{field: FieldPriority, text: "High"}), "閲覧のみ")
	c.readOnly = false
	wantReason(t, judge(c, parsed{field: FieldPriority, text: "High"}), "削除済み")
}

func TestJudgeKinds(t *testing.T) {
	c := dropKind(jctx(item.Epic), "priority", item.Epic)
	wantReason(t, judge(c, parsed{field: FieldPriority, text: "High"}), "Epic には優先度がない")
	wantReason(t, judge(jctx(item.Epic), parsed{field: FieldSprint, text: "S-1"}), "Epic はスプリントに入れない")
	wantReason(t, judge(jctx(item.Step), parsed{field: FieldRelease, text: "R-1"}), "Step はリリースに入れない")
	c = dropKind(jctx(item.Task), "systems", item.Task)
	wantReason(t, judge(c, parsed{field: FieldSystemsAdd, names: []string{"Web"}}), "Task には関連システムがない")
	wantReason(t, judge(c, parsed{field: FieldSystemsRemove, names: []string{"Web"}}), "Task には関連システムがない")
	c = editProperty(jctx(item.Epic), "priority", func(p *workspace.Property) { p.Hidden = true })
	wantDrafts(t, judge(c, parsed{field: FieldPriority, text: "High"}),
		change.NewSet("priority", c.it.Priority, valueID(c.settings.Priorities, "High")))
}

func TestJudgeNameIgnoresCase(t *testing.T) {
	c := jctx(item.Task)
	wantDrafts(t, judge(c, parsed{field: FieldStatus, text: "done"}),
		change.NewSet("status", c.it.Status, valueID(c.settings.Statuses, "Done")))
	wantReason(t, judge(c, parsed{field: FieldStatus, text: "Closed"}), "値がない")
}

func TestJudgeSame(t *testing.T) {
	c := jctx(item.Task)
	wantSame(t, judge(c, parsed{field: FieldStatus, text: "Todo"}))
	web, api := valueID(c.settings.Systems, "Web"), valueID(c.settings.Systems, "API")
	c.it.Systems = []change.ID{web, api}
	wantSame(t, judge(c, parsed{field: FieldSystemsAdd, names: []string{"Web", "API"}}))
	c.it.Systems = []change.ID{web}
	wantDrafts(t, judge(c, parsed{field: FieldSystemsAdd, names: []string{"Web", "API"}}), change.NewAdd("systems", api))
	wantDrafts(t, judge(c, parsed{field: FieldSystemsRemove, names: []string{"Web", "API"}}), change.NewRemove("systems", web))
	c.it.Systems = nil
	wantSame(t, judge(c, parsed{field: FieldSystemsRemove, names: []string{"Web", "API"}}))
	wantReason(t, judge(c, parsed{field: FieldSystemsAdd, names: []string{"Web", "Nope"}}), "値がない")
}

func TestJudgeCategory(t *testing.T) {
	c := jctx(item.Task)
	done := c.settings.CategoryDefaults[workspace.CategoryDone]
	in := parsed{field: FieldStatus, category: workspace.CategoryDone}
	c.it.Status = done
	wantSame(t, judge(c, in))
	todo, cancel := valueID(c.settings.Statuses, "Todo"), valueID(c.settings.Statuses, "Cancel")
	c.it.Status = todo
	wantDrafts(t, judge(c, in), change.NewSet("status", todo, done))
	c.it.Status = cancel
	wantDrafts(t, judge(c, in), change.NewSet("status", cancel, done))
}

func TestJudgeStatusRules(t *testing.T) {
	c := jctx(item.Task)
	done := valueID(c.settings.Statuses, "Done")
	c.rules = projectrule.Rules{Hidden: []change.ID{done}}
	c.settings.CategoryDefaults[workspace.CategoryDone] = valueID(c.settings.Statuses, "Cancel") // Done を既定から外す
	wantReason(t, judge(c, parsed{field: FieldStatus, text: "Done"}), "このプロジェクトでは使わないステータス")

	c = jctx(item.Task)
	todo, doing := valueID(c.settings.Statuses, "Todo"), valueID(c.settings.Statuses, "InProgress")
	c.settings.Workflow = map[change.ID][]change.ID{todo: {doing}}
	want := projectrule.CheckTransition(c.settings, c.rules, todo, done).Error()
	wantReason(t, judge(c, parsed{field: FieldStatus, text: "Done"}), want)
	wantReason(t, judge(c, parsed{field: FieldStatus, category: workspace.CategoryDone}), want)
}

func TestJudgeAssignee(t *testing.T) {
	c := jctx(item.Task)
	wantDrafts(t, judge(c, parsed{field: FieldAssignee, text: "HANAKO"}), change.NewSet("assignee", nil, "hanako"))
	wantReason(t, judge(c, parsed{field: FieldAssignee, text: "jiro"}), "担当者がこの Store のメンバーにいません")
	c.it.Assignee = "hanako"
	wantDrafts(t, judge(c, parsed{field: FieldAssignee}), change.NewSet("assignee", "hanako", nil))
	wantSame(t, judge(c, parsed{field: FieldAssignee, text: "hanako"}))
}

func TestJudgeSprintRelease(t *testing.T) {
	c := jctx(item.Task)
	got := judge(c, parsed{field: FieldSprint, text: "s-1"})
	wantDrafts(t, got, change.NewSet(item.SprintField, nil, "SP1"))
	if got.sprint == nil || *got.sprint != "SP1" {
		t.Fatalf("sprint = %v", got.sprint)
	}
	wantReason(t, judge(c, parsed{field: FieldSprint, text: "Nope"}), "値がない")
	wantReason(t, judge(c, parsed{field: FieldSprint, text: "S-2"}), "値がない") // 完了したスプリントには入れない
	wantReason(t, judge(c, parsed{field: FieldSprint, text: "S-3"}), "値がない") // 削除済み
	c.it.Sprint = "SP3"
	wantSame(t, judge(c, parsed{field: FieldSprint})) // 使えないスプリントへの参照はバックログと同じ
	c.it.Sprint = "SP1"
	got = judge(c, parsed{field: FieldSprint})
	wantDrafts(t, got, change.NewSet(item.SprintField, "SP1", nil))
	if got.sprint == nil || *got.sprint != "" {
		t.Fatalf("sprint = %v", got.sprint)
	}
	c.it.Sprint = ""
	wantDrafts(t, judge(c, parsed{field: FieldRelease, text: "R-1"}), change.NewSet(item.ReleaseField, nil, "R1"))
	wantReason(t, judge(c, parsed{field: FieldRelease, text: "Nope"}), "値がない")
	c.it.Release = "R9"
	wantSame(t, judge(c, parsed{field: FieldRelease}))
}

func TestJudgeDates(t *testing.T) {
	c := jctx(item.Task)
	c.it.End = "2026-11-10"
	wantReason(t, judge(c, parsed{field: FieldStart, date: "2026-11-12"}), "終了日が開始日より前になる")
	c.it.Start = "2026-11-01"
	wantDrafts(t, judge(c, parsed{field: FieldStart}), change.NewSet("start", "2026-11-01", nil))
	wantDrafts(t, judge(c, parsed{field: FieldEnd, date: "2026-11-20"}), change.NewSet("end", "2026-11-10", "2026-11-20"))
	wantReason(t, judge(c, parsed{field: FieldEnd, date: "2026-10-20"}), "終了日が開始日より前になる")
	wantSame(t, judge(c, parsed{field: FieldEnd, date: "2026-11-10"}))
}

func TestJudgeNumbers(t *testing.T) {
	c := jctx(item.Task)
	three := 3.0
	wantDrafts(t, judge(c, parsed{field: FieldPoints, points: &three}), change.NewSet("points", nil, &three))
	c.it.Points = &three
	wantSame(t, judge(c, parsed{field: FieldPoints, points: &three}))
	wantDrafts(t, judge(c, parsed{field: FieldPoints}), change.NewSet("points", &three, nil))
	wantDrafts(t, judge(c, parsed{field: FieldEstimate, estimate: &three}), change.NewSet("estimate", nil, &three))
}
