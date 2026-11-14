package repair_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func val(id string) workspace.Value { return workspace.Value{ID: change.ID(id), Name: id} }

// valueSettings は、ステータス s1・優先度 p1・システム y1・単一選択の追加プロパティ c(選択肢 v1)・ユーザー型 u を持つ設定。
func valueSettings() workspace.Settings {
	return workspace.Settings{
		Statuses: []workspace.Value{val("s1")}, Priorities: []workspace.Value{val("p1")},
		Systems: []workspace.Value{val("y1")},
		Properties: []workspace.Property{
			{ID: "c", Type: workspace.TypeSingle, Choices: []workspace.Value{val("v1")}},
			{ID: "u", Type: workspace.TypeUser},
		},
	}
}

// leaf は、問題のない Task(親 e)。with で値を変える。
func leaf(id string, n int, with func(*item.Item)) item.Item {
	it := inWS(id, "w", "p1", n, item.Task, "e")
	it.Status = "s1"
	if with != nil {
		with(&it)
	}
	return it
}

func valueState(members []string, plan planning.Plan, items ...item.Item) repair.StoreState {
	ms := make([]member.Member, 0, len(members))
	for _, l := range members {
		ms = append(ms, member.Member{Login: l})
	}
	epic := inWS("e", "w", "p1", 1, item.Epic, "")
	epic.Status = "s1"
	return repair.StoreState{
		Items:    append([]item.Item{epic}, items...),
		Settings: map[workspace.ID]workspace.Settings{"w": valueSettings()},
		Plans:    map[workspace.ID]planning.Plan{"w": plan}, Members: ms,
	}
}

func check(t *testing.T, s repair.StoreState, want map[item.ID][]repair.Problem) {
	t.Helper()
	wantProblems(t, repair.ItemProblems(s), want)
}

func TestNoStatus(t *testing.T) {
	s := valueState(nil, planning.Plan{},
		leaf("ok", 2, nil),
		leaf("none", 3, func(it *item.Item) { it.Status = "" }),
		leaf("other", 4, func(it *item.Item) { it.Status = "gone" }),
		gone(leaf("del", 5, func(it *item.Item) { it.Status = "gone" })),
	)
	check(t, s, map[item.ID][]repair.Problem{"none": {repair.ProblemNoStatus}, "other": {repair.ProblemNoStatus}})
}

func TestMissingValue(t *testing.T) {
	s := valueState(nil, planning.Plan{},
		leaf("ok", 2, func(it *item.Item) {
			it.Priority, it.Systems, it.Choices = "p1", []change.ID{"y1"}, map[change.ID][]change.ID{"c": {"v1"}}
		}),
		leaf("empty-priority", 3, nil),
		leaf("prio", 4, func(it *item.Item) { it.Priority = "gone" }),
		leaf("sys", 5, func(it *item.Item) { it.Systems = []change.ID{"y1", "gone"} }),
		leaf("choice", 6, func(it *item.Item) { it.Choices = map[change.ID][]change.ID{"c": {"gone"}} }),
		leaf("prop", 7, func(it *item.Item) { it.Choices = map[change.ID][]change.ID{"deleted-prop": {"v1"}} }),
	)
	bad := []repair.Problem{repair.ProblemMissingValue}
	check(t, s, map[item.ID][]repair.Problem{"prio": bad, "sys": bad, "choice": bad, "prop": bad})
}

func TestMissingPerson(t *testing.T) {
	user := func(login string) map[change.ID]json.RawMessage {
		raw, _ := json.Marshal(login)
		return map[change.ID]json.RawMessage{"u": raw}
	}
	s := valueState([]string{"Alice"}, planning.Plan{},
		leaf("ok", 2, func(it *item.Item) { it.Assignee, it.Fields = "alice", user("ALICE") }),
		leaf("unassigned", 3, nil),
		leaf("assignee", 4, func(it *item.Item) { it.Assignee = "bob" }),
		leaf("field", 5, func(it *item.Item) { it.Fields = user("bob") }),
	)
	bad := []repair.Problem{repair.ProblemMissingPerson}
	check(t, s, map[item.ID][]repair.Problem{"assignee": bad, "field": bad})
}

func TestMissingPlan(t *testing.T) {
	plan := planning.Plan{
		Sprints:  []planning.Sprint{{ID: "sp1"}, {ID: "sp-del", Deleted: &change.Deletion{Op: "op"}}},
		Releases: []planning.Release{{ID: "r1"}},
	}
	s := valueState(nil, plan,
		leaf("ok", 2, func(it *item.Item) { it.Sprint, it.Release = "sp1", "r1" }),
		leaf("deleted-sprint", 3, func(it *item.Item) { it.Sprint = "sp-del" }),
		leaf("sprint", 4, func(it *item.Item) { it.Sprint = "gone" }),
		leaf("release", 5, func(it *item.Item) { it.Release = "gone" }),
	)
	bad := []repair.Problem{repair.ProblemMissingPlan}
	check(t, s, map[item.ID][]repair.Problem{"sprint": bad, "release": bad})
}

func TestDanglingRelation(t *testing.T) {
	rel := func(to string) func(*item.Item) {
		return func(it *item.Item) { it.Relations = []item.Relation{{Type: "relates", To: item.ID(to)}} }
	}
	s := valueState(nil, planning.Plan{},
		leaf("ok", 2, rel("e")),
		gone(leaf("del", 3, nil)),
		leaf("to-deleted", 4, rel("del")),
		leaf("dangling", 5, rel("nowhere")),
	)
	check(t, s, map[item.ID][]repair.Problem{"dangling": {repair.ProblemDanglingRelation}})
}

func TestDateOrder(t *testing.T) {
	dates := func(start, end string) func(*item.Item) {
		return func(it *item.Item) { it.Start, it.End = start, end }
	}
	s := valueState(nil, planning.Plan{},
		leaf("ok", 2, dates("2026-11-01", "2026-11-02")),
		leaf("same", 3, dates("2026-11-01", "2026-11-01")),
		leaf("start-only", 4, dates("2026-11-01", "")),
		leaf("end-only", 5, dates("", "2026-11-01")),
		leaf("reversed", 6, dates("2026-11-02", "2026-11-01")),
	)
	check(t, s, map[item.ID][]repair.Problem{"reversed": {repair.ProblemDateOrder}})
}

// 値の問題と親子の問題が両方あるアイテムは、Problem の宣言の順に返る。
func TestProblemOrderMixed(t *testing.T) {
	s := valueState(nil, planning.Plan{},
		inWS("x", "w", "p1", 2, item.Story, ""),
		inWS("y", "w", "p2", 3, item.Task, "e"),
	)
	s.Items[1].Status = "gone"
	s.Items[1].Start, s.Items[1].End = "2026-11-02", "2026-11-01"
	s.Items[2].Status = "gone"
	check(t, s, map[item.ID][]repair.Problem{
		"x": {repair.ProblemNoParent, repair.ProblemNoStatus, repair.ProblemDateOrder},
		"y": {repair.ProblemNoStatus, repair.ProblemParentElsewhere},
	})
}
