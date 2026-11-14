package repair_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// at は、ワークスペース ws・プロジェクト pr のアイテム。parent が空なら親なし。
func at(id, ws string, pr project.ID, parent string) item.Item {
	it := node(id, 1, item.Task, parent)
	it.Place.Workspace = workspace.ID(ws)
	it.Place.Project = pr
	return it
}

func TestProjectMismatch(t *testing.T) {
	items := []item.Item{at("a", "w1", "p2", ""), at("b", "w1", "p1", "a")}
	got, ok := repair.ProjectMismatch(items)
	if !ok || got != (repair.Mismatch{Child: "b", Project: "p2"}) {
		t.Errorf("ProjectMismatch = %v %v, want {b p2}", got, ok)
	}
}

// 孫も違うときは、深さの浅い子が先。
func TestProjectMismatchShallowFirst(t *testing.T) {
	items := []item.Item{at("a", "w1", "p2", ""), at("c", "w1", "p1", "b"), at("b", "w1", "p1", "a")}
	got, ok := repair.ProjectMismatch(items)
	if !ok || got.Child != "b" {
		t.Errorf("ProjectMismatch = %v %v, want child b", got, ok)
	}
}

// 同じ深さの子が複数あるときは、内部 ID の小さいものが先。
func TestProjectMismatchSameDepthByID(t *testing.T) {
	items := []item.Item{at("a", "w1", "p2", ""), at("z", "w1", "p1", "a"), at("c", "w1", "p1", "a")}
	got, ok := repair.ProjectMismatch(items)
	if !ok || got.Child != "c" {
		t.Errorf("ProjectMismatch = %v %v, want child c", got, ok)
	}
}

// 親が輪になっていても止まる。深さは同じ(1)なので、内部 ID の小さい a を、親 b のプロジェクトへ移す。
func TestProjectMismatchCyclicParents(t *testing.T) {
	items := []item.Item{at("b", "w1", "p2", "a"), at("a", "w1", "p1", "b")}
	got, ok := repair.ProjectMismatch(items)
	if !ok || got != (repair.Mismatch{Child: "a", Project: "p2"}) {
		t.Errorf("ProjectMismatch = %v %v, want {a p2}", got, ok)
	}
}

func TestProjectMismatchSkips(t *testing.T) {
	deleted := func(it item.Item) item.Item {
		it.Deleted = &change.Deletion{}
		return it
	}
	cases := map[string][]item.Item{
		"別のワークスペース": {at("a", "w1", "p2", ""), at("b", "w2", "p1", "a")},
		"削除済みの子":    {at("a", "w1", "p2", ""), deleted(at("b", "w1", "p1", "a"))},
		"削除済みの親":    {deleted(at("a", "w1", "p2", "")), at("b", "w1", "p1", "a")},
		"同じプロジェクト":  {at("a", "w1", "p1", ""), at("b", "w1", "p1", "a")},
		"親が見つからない":  {at("b", "w1", "p1", "zz")},
	}
	for name, items := range cases {
		if got, ok := repair.ProjectMismatch(items); ok {
			t.Errorf("%s: ProjectMismatch = %v, want none", name, got)
		}
	}
}

// wsAt は、ワークスペース ws・プロジェクト pr・番号 n のアイテム(削除済みにもできる)。
func wsAt(id, ws string, pr project.ID, n int, parent string, deleted bool) item.Item {
	it := at(id, ws, pr, parent)
	it.Place.Number = n
	if deleted {
		it.Deleted = &change.Deletion{}
	}
	return it
}

func TestWorkspaceMismatchFindsChildInOtherWorkspace(t *testing.T) {
	items := []item.Item{wsAt("e", "w2", "p2", 1, "", false), wsAt("t", "w1", "p1", 2, "e", false)}
	got, ok := repair.WorkspaceMismatch(items)
	if !ok || got != (repair.Stray{Child: "t", Workspace: "w2", Project: "p2"}) {
		t.Errorf("WorkspaceMismatch = %v %v, want {t w2 p2}", got, ok)
	}
}

// 同じワークスペースでプロジェクトだけ違うものは、ProjectMismatch の持ち場。
func TestWorkspaceMismatchIgnoresSameWorkspace(t *testing.T) {
	items := []item.Item{wsAt("e", "w1", "p2", 1, "", false), wsAt("t", "w1", "p1", 2, "e", false)}
	if got, ok := repair.WorkspaceMismatch(items); ok {
		t.Errorf("WorkspaceMismatch = %v, want none", got)
	}
}

func TestWorkspaceMismatchIgnoresDeleted(t *testing.T) {
	cases := map[string][]item.Item{
		"削除済みの子": {wsAt("e", "w2", "p2", 1, "", false), wsAt("t", "w1", "p1", 2, "e", true)},
		"削除済みの親": {wsAt("e", "w2", "p2", 1, "", true), wsAt("t", "w1", "p1", 2, "e", false)},
	}
	for name, items := range cases {
		if got, ok := repair.WorkspaceMismatch(items); ok {
			t.Errorf("%s: WorkspaceMismatch = %v, want none", name, got)
		}
	}
}

func TestWorkspaceMismatchPicksShallowest(t *testing.T) {
	items := []item.Item{
		wsAt("e", "w2", "p2", 1, "", false),
		wsAt("s", "w3", "p3", 1, "t", false),
		wsAt("t", "w1", "p1", 9, "e", false),
	}
	got, ok := repair.WorkspaceMismatch(items)
	if !ok || got.Child != "t" {
		t.Errorf("WorkspaceMismatch = %v %v, want child t", got, ok)
	}
}

func TestWorkspaceMismatchPicksSmallerOldNumber(t *testing.T) {
	items := []item.Item{
		wsAt("e", "w2", "p2", 1, "", false),
		wsAt("t5", "w1", "p1", 5, "e", false),
		wsAt("t3", "w1", "p1", 3, "e", false),
	}
	if got, ok := repair.WorkspaceMismatch(items); !ok || got.Child != "t3" {
		t.Errorf("WorkspaceMismatch = %v %v, want child t3", got, ok)
	}
	// 番号が同じなら内部 ID の小さいほう。
	items = []item.Item{
		wsAt("e", "w2", "p2", 1, "", false),
		wsAt("z", "w1", "p1", 4, "e", false),
		wsAt("c", "w1", "p1", 4, "e", false),
	}
	if got, ok := repair.WorkspaceMismatch(items); !ok || got.Child != "c" {
		t.Errorf("WorkspaceMismatch = %v %v, want child c", got, ok)
	}
}

// 削除済みだけをたどる。削除されていない s3 は自分で直す子になるので含めない。
func TestDeletedBelowFollowsOnlyDeleted(t *testing.T) {
	items := []item.Item{
		wsAt("t", "w1", "p1", 1, "", false),
		wsAt("s1", "w1", "p1", 2, "t", true),
		wsAt("s2", "w1", "p1", 3, "s1", true),
		wsAt("s3", "w1", "p1", 4, "t", false),
		wsAt("s4", "w1", "p1", 5, "s3", true),
	}
	var ids []item.ID
	for _, it := range repair.DeletedBelow("t", items) {
		ids = append(ids, it.ID)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []item.ID{"s1", "s2"}) {
		t.Errorf("DeletedBelow = %v, want [s1 s2]", ids)
	}
}

// 削除済みの親が輪になっていても止まる。t と b が互いの親(t は輪の中)で、a は b の子。
// t から b、a の順に届き、b から t に戻るところで止まる。a・b は 1 回ずつ、t 自身は含まない。
// 実際の呼び出しでは child は削除されていないので輪には届かない。seen は守りとして確かめる。
func TestDeletedBelowStopsOnCycle(t *testing.T) {
	items := []item.Item{
		wsAt("t", "w1", "p1", 1, "b", true),
		wsAt("b", "w1", "p1", 2, "t", true),
		wsAt("a", "w1", "p1", 3, "b", true),
	}
	var ids []item.ID
	for _, it := range repair.DeletedBelow("t", items) {
		ids = append(ids, it.ID)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []item.ID{"a", "b"}) {
		t.Errorf("DeletedBelow = %v, want [a b]", ids)
	}
}

func TestNumberByOldUsesOldNumbers(t *testing.T) {
	child := wsAt("t", "w1", "p1", 7, "", false)
	s9 := wsAt("s9", "w1", "p1", 9, "t", true)
	s4 := wsAt("s4", "w1", "p1", 4, "t", true)
	deleted := []item.Item{s9, s4}
	got := repair.NumberByOld(child, deleted, 10)
	want := map[item.ID]int{"t": 10, "s4": 11, "s9": 12}
	if !maps.Equal(got, want) {
		t.Errorf("NumberByOld = %v, want %v", got, want)
	}
	if deleted[0].ID != "s9" {
		t.Error("NumberByOld changed its argument")
	}
	// 番号が同じものは内部 ID の順。
	x, y := wsAt("y", "w1", "p1", 4, "t", true), wsAt("x", "w1", "p1", 4, "t", true)
	got = repair.NumberByOld(child, []item.Item{x, y}, 1)
	if got["x"] != 2 || got["y"] != 3 {
		t.Errorf("NumberByOld = %v, want x:2 y:3", got)
	}
}
