package repair_test

import (
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
