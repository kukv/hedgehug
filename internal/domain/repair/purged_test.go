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

func placed(id, proj string, parent string, rels ...item.Relation) item.Item {
	it := item.Item{ID: item.ID(id), Place: item.Place{Project: project.ID(proj)}, Relations: rels}
	if parent != "" {
		p := item.ID(parent)
		it.Place.Parent = &p
	}
	return it
}

func TestUnderPurged(t *testing.T) {
	pw := map[project.ID]workspace.ID{"P1": "W1", "P2": "W2"}
	// S1 は T1 の下、T1 は E1 の下。並びは子が先(内部 ID の順で親より前に来るもの)。
	items := []item.Item{
		placed("A0", "P1", "T1"), // Step: 親 T1 は E1 の下
		placed("E1", "P1", ""),
		placed("T1", "P1", "E1"),
		placed("X1", "P1", ""),
		placed("Y1", "P2", ""),
		placed("Z1", "P3", ""), // projectWorkspace にないプロジェクト
	}
	tests := []struct {
		name   string
		items  []item.Item
		purged map[change.ID]bool
		want   []item.ID
	}{
		{"parent purged", items, idSet("E1"), []item.ID{"A0", "T1"}},
		{"parent folder gone", items[:1], idSet("T1"), []item.ID{"A0"}},
		{"grandparent purged, parent gone", []item.Item{placed("A0", "P1", "T1")}, idSet("E1"), nil},
		{"project purged", items, idSet("P1"), []item.ID{"A0", "E1", "T1", "X1"}},
		{"workspace purged", items, idSet("W2"), []item.ID{"Y1"}},
		{"unknown project's workspace not seen", items, idSet("W3"), nil},
		{"project not in map, project purged", items, idSet("P3"), []item.ID{"Z1"}},
		{"unrelated", items, idSet("Q9"), nil},
		{"cycle", []item.Item{placed("C1", "P1", "C2"), placed("C2", "P1", "C1")}, idSet("Q9"), nil},
		{"cycle under purged", []item.Item{placed("C1", "P1", "C2"), placed("C2", "P1", "C1")}, idSet("P1"), []item.ID{"C1", "C2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repair.UnderPurged(tt.items, pw, tt.purged)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPurgedRelations(t *testing.T) {
	blocks := func(to string) item.Relation { return item.Relation{Type: "blocks", To: item.ID(to)} }
	relates := func(to string) item.Relation { return item.Relation{Type: "relates", To: item.ID(to)} }
	items := []item.Item{
		placed("B1", "P1", "", relates("G1"), blocks("G1"), blocks("K1")),
		placed("A1", "P1", "", relates("G2"), relates("G1")),
		placed("C1", "P1", "", blocks("K1")),
	}
	got := repair.PurgedRelations(items, idSet("G1", "G2"))
	want := []repair.Dangling{
		{Owner: "A1", Relation: relates("G1")},
		{Owner: "A1", Relation: relates("G2")},
		{Owner: "B1", Relation: blocks("G1")},
		{Owner: "B1", Relation: relates("G1")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if got := repair.PurgedRelations(items, idSet("Q9")); got != nil {
		t.Fatalf("unrelated: %v", got)
	}
}
