package repair_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

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
	otherFRestore := row("r2", "op3", change.Restore, "statuses/Z", user)
	otherFRestore.To = json.RawMessage(`"op1"`)
	var overlapNone []repair.Overlap
	wsDel := del("d3", "op1", "statuses/X", true)
	always := func(string, change.Record) bool { return true }
	overlap1 := []repair.Overlap{{Op: "op1", F: "deleted"}}
	tests := []struct {
		name       string
		log        []change.Record
		ours       map[change.ID]bool
		theirs     map[change.ID]bool
		incidental func(string, change.Record) bool
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
		{"change on both sides", []change.Record{itemDel, title}, idSet("d1", "c1"), idSet("c1"), nil, overlapNone},
		{
			"restore with different f",
			[]change.Record{itemDel, title, otherFRestore},
			idSet("d1"), idSet("c1"), nil, overlap1,
		},
		{
			"two deletes in one op",
			[]change.Record{
				del("d5", "op1", "statuses/B", true), del("d4", "op1", "statuses/A", true),
				row("c7", "op2", change.Set, "statuses/A/name", user),
				row("c8", "op2", change.Set, "statuses/B/name", user),
			},
			idSet("d4", "d5"), idSet("c7", "c8"), nil,
			[]repair.Overlap{{Op: "op1", F: "statuses/A"}},
		},
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

func TestChanger(t *testing.T) {
	at := func(r change.Record, by string, sec int) change.Record {
		r.By, r.At = by, time.Date(2026, 11, 14, 9, 0, sec, 0, time.UTC)
		return r
	}
	user := change.ActorUser
	itemDel := at(del("d1", "op1", "deleted", true), "taro", 5)
	valueDel := at(del("d2", "op1", "statuses/X", true), "taro", 5)
	tests := []struct {
		name string
		log  []change.Record
		d    change.Record
		want string
	}{
		{"last human row", []change.Record{
			at(row("c1", "op2", change.Set, "title", user), "hanako", 1), itemDel,
			at(row("c2", "op3", change.Set, "status", user), "jiro", 3),
		}, itemDel, "jiro"},
		{"later by at, not by order", []change.Record{
			at(row("c2", "op3", change.Set, "status", user), "jiro", 3),
			at(row("c1", "op2", change.Set, "title", user), "hanako", 1), itemDel,
		}, itemDel, "jiro"},
		{"same at: larger id", []change.Record{
			at(row("c2", "op3", change.Set, "status", user), "jiro", 3),
			at(row("c1", "op2", change.Set, "title", user), "hanako", 3), itemDel,
		}, itemDel, "jiro"},
		{"same op not counted", []change.Record{
			at(row("c1", "op2", change.Set, "title", user), "hanako", 1),
			at(row("c3", "op1", change.Set, "rank", user), "taro", 5), itemDel,
		}, itemDel, "hanako"},
		{"sync and automation not counted", []change.Record{
			at(row("c1", "op2", change.Set, "title", user), "hanako", 1), itemDel,
			at(row("c4", "op4", change.Set, "status", change.ActorAutomation), "jiro", 7),
			at(row("c5", "op5", change.Set, "place", change.ActorSync), "hedgehug", 8),
		}, itemDel, "hanako"},
		{"delete and restore not counted", []change.Record{
			at(row("c1", "op2", change.Set, "title", user), "hanako", 1), itemDel,
			at(row("r1", "op6", change.Restore, "deleted", user), "jiro", 9),
			at(del("d9", "op7", "deleted", true), "jiro", 9),
		}, itemDel, "hanako"},
		{"value: f and below only", []change.Record{
			at(row("c6", "op2", change.Set, "statuses/X/name", user), "hanako", 1), valueDel,
			at(row("c7", "op3", change.Set, "statuses/XY/name", user), "jiro", 6),
			at(row("c8", "op4", change.Set, "name", user), "jiro", 7),
		}, valueDel, "hanako"},
		{"same f", []change.Record{at(row("c9", "op2", change.Comment, "statuses/X", user), "hanako", 1), valueDel}, valueDel, "hanako"},
		{"create and moved_in not counted", []change.Record{
			at(row("c10", "op0", change.Create, "", user), "jiro", 0),
			at(row("c11", "op8", change.MovedIn, "", user), "jiro", 0), itemDel,
		}, itemDel, ""},
		{"none", []change.Record{itemDel}, itemDel, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repair.Changer(tt.log, tt.d); got != tt.want {
				t.Errorf("Changer = %q, want %q", got, tt.want)
			}
		})
	}
}
