package search

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// relationSnapshot は、1 つのワークスペース(接頭辞 HH)に、画面に出る A(HH-1、内部 ID z-a)・B(HH-2、b)・C(HH-3、c)と、
// 削除済みの D(HH-4)・E(HH-5)を置いた材料。A は B をブロックし、B と互いに relates、自分にも relates、D をブロックする。
// C は A を複製し、D は B をブロックし E を引き起こし、E は A と重複する。
func relationSnapshot(deleted bool) *snapshot {
	rel := func(typ string, to item.ID) item.Relation { return item.Relation{Type: typ, To: to} }
	mk := func(id item.ID, n int, rels ...item.Relation) item.Item {
		return item.Item{ID: id, Place: item.Place{Workspace: "w", Project: "p", Number: n}, Relations: rels}
	}
	gone := &change.Deletion{Op: "op"}
	a := mk("z-a", 1, rel("blocks", "b"), rel("relates", "b"), rel("relates", "z-a"), rel("blocks", "d"))
	b := mk("b", 2, rel("relates", "z-a"))
	c := mk("c", 3, rel("clones", "z-a"))
	d := mk("d", 4, rel("blocks", "b"), rel("causes", "e"))
	d.Deleted = gone
	e := mk("e", 5, rel("duplicates", "z-a"))
	e.Deleted = gone
	s := &snapshot{
		spaces:   map[workspace.ID]*space{"w": {ws: workspace.Workspace{ID: "w", Prefix: "HH"}}},
		projects: map[project.ID]project.Project{"p": {ID: "p"}},
		items:    []item.Item{a, b, c, d, e},
		byID:     map[item.ID]item.Item{},
	}
	for _, it := range s.items {
		s.byID[it.ID] = it
	}
	s.pick(deleted)
	return s
}

func refs(displays ...string) []query.Ref {
	out := make([]query.Ref, 0, len(displays))
	for _, d := range displays {
		out = append(out, query.Ref{Display: d})
	}
	return out
}

// relations は、両方向の表記を入れ、同じ相手は 1 つにし、相手の内部 ID の順に並べ、削除済みの相手を除く。
// 自分への関連は持つ側の表記だけ。削除済みのアイテム自身は、自分が持つ関連と、画面に出るアイテムからの関連を見る。
func TestRelations(t *testing.T) {
	shown := relationSnapshot(false)
	want := map[item.ID]map[string][]query.Ref{
		"z-a": {"blocks": refs("HH-2"), "relates_to": refs("HH-2", "HH-1"), "cloned_by": refs("HH-3")},
		"b":   {"blocked_by": refs("HH-1"), "relates_to": refs("HH-1")},
		"c":   {"clones": refs("HH-1")},
	}
	for id, w := range want {
		if got := shown.relations(shown.byID[id], nil); !reflect.DeepEqual(got, w) {
			t.Errorf("%s: %+v, want %+v", id, got, w)
		}
	}
	del := relationSnapshot(true)
	wantDel := map[item.ID]map[string][]query.Ref{
		"d": {"blocks": refs("HH-2"), "blocked_by": refs("HH-1")},
		"e": {"duplicates": refs("HH-1")},
	}
	for id, w := range wantDel {
		if got := del.relations(del.byID[id], nil); !reflect.DeepEqual(got, w) {
			t.Errorf("deleted %s: %+v, want %+v", id, got, w)
		}
	}
}
