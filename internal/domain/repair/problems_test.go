package repair_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// inWS は、ワークスペース ws・プロジェクト pj のアイテム。parent が空なら親なし。
func inWS(id, ws, pj string, n int, kind item.Kind, parent string) item.Item {
	it := item.Item{ID: item.ID(id), Kind: kind, Place: item.Place{
		Workspace: workspace.ID(ws), Project: project.ID(pj), Number: n,
	}}
	if parent != "" {
		p := item.ID(parent)
		it.Place.Parent = &p
	}
	return it
}

func gone(it item.Item) item.Item {
	it.Deleted = &change.Deletion{Op: "op"}
	return it
}

func problems(items ...item.Item) map[item.ID][]repair.Problem {
	return repair.ItemProblems(repair.StoreState{Items: items})
}

func wantProblems(t *testing.T, got, want map[item.ID][]repair.Problem) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ItemProblems = %v, want %v", got, want)
	}
}

func TestNoParent(t *testing.T) {
	got := problems(
		inWS("e", "w", "p1", 1, item.Epic, ""),
		inWS("s", "w", "p1", 2, item.Story, ""),
		gone(inWS("d", "w", "p1", 3, item.Epic, "")),
		inWS("t", "w", "p1", 4, item.Task, "d"),
		inWS("x", "w", "p1", 5, item.Step, "nowhere"),
	)
	wantProblems(t, got, map[item.ID][]repair.Problem{
		"s": {repair.ProblemNoParent}, "t": {repair.ProblemNoParent}, "x": {repair.ProblemNoParent},
	})
}

func TestBadParentKind(t *testing.T) {
	got := problems(
		inWS("e", "w", "p1", 1, item.Epic, ""),
		inWS("s1", "w", "p1", 2, item.Story, "e"),
		inWS("s2", "w", "p1", 3, item.Story, "s1"),
		inWS("st", "w", "p1", 4, item.Step, "e"),
		inWS("ep", "w", "p1", 5, item.Epic, "s1"),
		inWS("eo", "w", "p1", 6, item.Epic, "nowhere"),
		inWS("ok", "w", "p1", 7, item.Step, "s1"),
	)
	wantProblems(t, got, map[item.ID][]repair.Problem{
		"s2": {repair.ProblemBadParentKind}, "st": {repair.ProblemBadParentKind},
		"ep": {repair.ProblemBadParentKind}, "eo": {repair.ProblemBadParentKind},
	})
}

// 輪の中は、種類の決まりに合わない親子が必ず含まれる(Epic に親はなく、Epic の下は作業項目だけ)。
// 輪の外から輪に入る c は、輪には入らない。
func TestParentCycle(t *testing.T) {
	got := problems(
		inWS("a", "w", "p1", 1, item.Task, "b"),
		inWS("b", "w", "p1", 2, item.Task, "a"),
		inWS("c", "w", "p1", 3, item.Step, "a"),
	)
	wantProblems(t, got, map[item.ID][]repair.Problem{
		"a": {repair.ProblemBadParentKind, repair.ProblemParentCycle},
		"b": {repair.ProblemBadParentKind, repair.ProblemParentCycle},
	})
}

func TestParentElsewhere(t *testing.T) {
	got := problems(
		inWS("e1", "w1", "p1", 1, item.Epic, ""),
		inWS("e2", "w1", "p2", 2, item.Epic, ""),
		inWS("t1", "w2", "p1", 1, item.Task, "e1"),
		inWS("t2", "w1", "p2", 3, item.Task, "e1"),
		inWS("t3", "w1", "p1", 4, item.Task, "e1"),
	)
	wantProblems(t, got, map[item.ID][]repair.Problem{
		"t1": {repair.ProblemParentElsewhere}, "t2": {repair.ProblemParentElsewhere},
	})
}

func TestNumberOverlap(t *testing.T) {
	got := problems(
		inWS("a", "w", "p1", 5, item.Epic, ""),
		inWS("b", "w", "p2", 5, item.Epic, ""),
		inWS("c", "w", "p1", 7, item.Epic, ""),
		gone(inWS("d", "w", "p1", 7, item.Epic, "")),
		inWS("o", "other", "p1", 5, item.Epic, ""),
		gone(inWS("x", "w", "p1", 9, item.Epic, "")),
		gone(inWS("y", "w", "p1", 9, item.Epic, "")),
	)
	wantProblems(t, got, map[item.ID][]repair.Problem{
		"a": {repair.ProblemNumberOverlap}, "b": {repair.ProblemNumberOverlap}, "c": {repair.ProblemNumberOverlap},
	})
}

func TestIssueTwice(t *testing.T) {
	items := []item.Item{
		inWS("a", "w", "p1", 1, item.Epic, ""), inWS("b", "w", "p1", 2, item.Epic, ""),
		inWS("c", "w", "p1", 3, item.Epic, ""), inWS("d", "w", "p1", 4, item.Epic, ""),
		gone(inWS("g", "w", "p1", 5, item.Epic, "")), inWS("h", "w", "p1", 6, item.Epic, ""),
	}
	issues := map[item.ID][]string{
		"a": {"o/r#1"}, "b": {"o/r#1"}, // 1 つの Issue に 2 件
		"c": {"o/r#2", "o/r#3"},        // 1 件に 2 つの Issue
		"d": {"o/r#4"}, "g": {"o/r#4"}, // 削除済みとの共有は数えない
		"h": {"o/r#5", "o/r#5"}, // 同じ鍵の重複は 1 つ
	}
	got := repair.ItemProblems(repair.StoreState{Items: items, Issues: issues})
	wantProblems(t, got, map[item.ID][]repair.Problem{
		"a": {repair.ProblemIssueTwice}, "b": {repair.ProblemIssueTwice}, "c": {repair.ProblemIssueTwice},
	})
}

// 削除済みのアイテムは、どの問題があっても結果に入れない。
func TestDeletedNeverReported(t *testing.T) {
	got := problems(gone(inWS("s", "w", "p1", 1, item.Story, "")), gone(inWS("t", "w", "p1", 1, item.Story, "x")))
	wantProblems(t, got, nil)
}

// 輪はすべて返す(Cycle は先頭の 1 つだけ)。
func TestRings(t *testing.T) {
	items := []item.Item{
		node("d", 1, item.Task, "c"), node("c", 2, item.Task, "d"),
		node("z", 3, item.Task, "b"), node("b", 4, item.Task, "z"),
	}
	want := [][]item.ID{{"b", "z"}, {"c", "d"}}
	if got := repair.Rings(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("Rings = %v, want %v", got, want)
	}
}
