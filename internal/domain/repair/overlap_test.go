package repair_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

func row(id, op string, t change.Type, f string, kind change.ActorKind) change.Record {
	return change.Record{ID: change.ID(id), Op: change.ID(op), T: t, F: f, Actor: change.Actor{Kind: kind}}
}

func del(id, op, f string, root bool) change.Record {
	r := row(id, op, change.Delete, f, change.ActorUser)
	r.Root = &root
	return r
}

func idSet(s ...string) map[change.ID]bool {
	m := map[change.ID]bool{}
	for _, id := range s {
		m[change.ID(id)] = true
	}
	return m
}

func TestOverlaps(t *testing.T) {
	user := change.ActorUser
	itemDel := del("d1", "op1", "deleted", true)
	title := row("c1", "op2", change.Set, "title", user)
	restore := row("r1", "op3", change.Restore, "deleted", user)
	restore.To = json.RawMessage(`"op1"`)
	wsDel := del("d3", "op1", "statuses/X", true)
	always := func(change.Record) bool { return true }
	overlap1 := []repair.Overlap{{Op: "op1", F: "deleted"}}
	tests := []struct {
		name       string
		log        []change.Record
		ours       map[change.ID]bool
		theirs     map[change.ID]bool
		incidental func(change.Record) bool
		want       []repair.Overlap
	}{
		{"delete ours, change theirs", []change.Record{itemDel, title}, idSet("d1"), idSet("c1"), nil, overlap1},
		{"delete theirs, change ours", []change.Record{itemDel, title}, idSet("c1"), idSet("d1"), nil, overlap1},
		{"same side", []change.Record{itemDel, title}, idSet("d1", "c1"), nil, nil, nil},
		{"same op row", []change.Record{itemDel, row("c3", "op1", change.Set, "title", user)}, idSet("d1"), idSet("c3"), nil, nil},
		{"root false", []change.Record{del("d2", "op1", "deleted", false), title}, idSet("d2"), idSet("c1"), nil, nil},
		{"already restored", []change.Record{itemDel, title, restore}, idSet("d1"), idSet("c1"), nil, nil},
		{"comment add", []change.Record{itemDel, row("c4", "op2", change.Comment, "", user)}, idSet("d1"), idSet("c4"), nil, overlap1},
		{
			"workspace statuses",
			[]change.Record{wsDel, row("c5", "op2", change.Set, "statuses/X/name", user)},
			idSet("d3"), idSet("c5"), nil,
			[]repair.Overlap{{Op: "op1", F: "statuses/X"}},
		},
		{
			"prefix needs slash",
			[]change.Record{wsDel, row("c5", "op2", change.Set, "statuses/XY/name", user)},
			idSet("d3"), idSet("c5"), nil, nil,
		},
		{"common ancestor", []change.Record{itemDel, title}, nil, nil, nil, nil},
		{
			"non-human actors",
			[]change.Record{
				itemDel,
				row("g1", "op2", change.Set, "link", change.ActorGitHub),
				row("g2", "op4", change.Set, "status", change.ActorAutomation),
				row("g3", "op5", change.Set, "place", change.ActorSync),
			},
			idSet("d1"), idSet("g1", "g2", "g3"), nil, nil,
		},
		{"incidental only", []change.Record{itemDel, title}, idSet("d1"), idSet("c1"), always, nil},
		{
			"sorted and deduped",
			[]change.Record{
				del("d9", "op9", "deleted", true), itemDel, title,
				row("c6", "op2", change.Set, "body", user),
			},
			idSet("d9", "d1"), idSet("c1", "c6"), nil,
			[]repair.Overlap{{Op: "op1", F: "deleted"}, {Op: "op9", F: "deleted"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repair.Overlaps(tt.log, repair.Facts{Ours: tt.ours, Theirs: tt.theirs}, tt.incidental)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Overlaps = %v, want %v", got, tt.want)
			}
		})
	}
}
