package relation_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

func rel(typ string, to item.ID) item.Relation { return item.Relation{Type: typ, To: to} }

func TestParseAndLabelOf(t *testing.T) {
	cases := []struct {
		label   relation.Label
		typ     string
		reverse bool
	}{
		{relation.Blocks, "blocks", false},
		{relation.BlockedBy, "blocks", true},
		{relation.Causes, "causes", false},
		{relation.CausedBy, "causes", true},
		{relation.Duplicates, "duplicates", false},
		{relation.DuplicatedBy, "duplicates", true},
		{relation.RelatesTo, "relates", false},
		{relation.Clones, "clones", false},
		{relation.ClonedBy, "clones", true},
	}
	for _, c := range cases {
		typ, reverse, ok := relation.Parse(c.label)
		if !ok || typ != c.typ || reverse != c.reverse {
			t.Errorf("Parse(%q) = %q, %v, %v", c.label, typ, reverse, ok)
		}
		if back, ok := relation.LabelOf(c.typ, c.reverse); !ok || back != c.label {
			t.Errorf("LabelOf(%q, %v) = %q, %v", c.typ, c.reverse, back, ok)
		}
	}
	if _, _, ok := relation.Parse("friends with"); ok {
		t.Error("知らない表記を通した")
	}
	if l, ok := relation.LabelOf("relates", true); !ok || l != relation.RelatesTo {
		t.Errorf("relates は向きがなく、持たれる側でも relates to: %q, %v", l, ok)
	}
	if _, ok := relation.LabelOf("friends", false); ok {
		t.Error("知らない種類を通した")
	}
}

func TestRowsShowBothSidesInLabelOrder(t *testing.T) {
	all := []item.Item{
		{ID: "A", Relations: []item.Relation{rel("blocks", "B"), rel("relates", "C"), rel("clones", "D")}},
		{ID: "B"},
		{ID: "C", Relations: []item.Relation{rel("relates", "A")}}, // 両側が持っても 1 行
		{ID: "D"},
		{ID: "E", Relations: []item.Relation{rel("blocks", "A"), rel("friends", "A")}}, // 知らない種類は読み飛ばす
		{ID: "F", Relations: []item.Relation{rel("blocks", "GONE")}},                   // 相手がいない関連は出さない
	}
	want := []relation.Row{
		{Label: relation.Blocks, Other: "B"},
		{Label: relation.BlockedBy, Other: "E"},
		{Label: relation.RelatesTo, Other: "C"},
		{Label: relation.Clones, Other: "D"},
	}
	if got := relation.Rows("A", all); !slices.Equal(got, want) {
		t.Errorf("Rows(A) = %+v, want %+v", got, want)
	}
	if got := relation.Rows("B", all); !slices.Equal(got, []relation.Row{{Label: relation.BlockedBy, Other: "A"}}) {
		t.Errorf("Rows(B) = %+v", got)
	}
	if got := relation.Rows("D", all); !slices.Equal(got, []relation.Row{{Label: relation.ClonedBy, Other: "A"}}) {
		t.Errorf("Rows(D) = %+v", got)
	}
	if got := relation.Rows("F", all); len(got) != 0 {
		t.Errorf("Rows(F) = %+v, want none", got)
	}
}

func TestHasIgnoresDirection(t *testing.T) {
	all := []item.Item{{ID: "A", Relations: []item.Relation{rel("blocks", "B")}}, {ID: "B"}}
	if !relation.Has("A", "B", "blocks", all) || !relation.Has("B", "A", "blocks", all) {
		t.Error("向きが逆でも同じ種類は 1 つと数える")
	}
	if relation.Has("A", "B", "causes", all) {
		t.Error("種類が違えば別")
	}
	if relation.Has("A", "C", "blocks", all) {
		t.Error("相手が違えば別")
	}
}

func TestWouldCycle(t *testing.T) {
	all := []item.Item{
		{ID: "A", Relations: []item.Relation{rel("blocks", "B")}},
		{ID: "B", Relations: []item.Relation{rel("blocks", "C"), rel("causes", "A")}},
		{ID: "C"},
	}
	cases := []struct {
		from, to item.ID
		want     bool
	}{
		{"C", "A", true},  // A→B→C に C→A を足すと輪
		{"A", "C", false}, // 近道を足すだけ
		{"B", "A", true},
		{"C", "B", true},
		{"A", "A", true}, // 自分自身
		{"C", "D", false},
	}
	for _, c := range cases {
		if got := relation.WouldCycle(c.from, c.to, all); got != c.want {
			t.Errorf("WouldCycle(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestBlocked(t *testing.T) {
	all := []item.Item{
		{ID: "A", Relations: []item.Relation{rel("blocks", "B"), rel("causes", "D")}},
		{ID: "C", Relations: []item.Relation{rel("blocks", "B"), rel("blocks", "E")}},
		{ID: "B"},
		{ID: "D"},
		{ID: "E"},
	}
	closed := func(it item.Item) bool { return it.ID == "C" } // C は完了
	got := relation.BlockedSet(all, closed)
	if !got["B"] || got["D"] || got["E"] || got["A"] {
		t.Errorf("BlockedSet = %v: B は A がまだ終わっていないのでブロック中、E は C が終わっているので外れ、causes は関係しない", got)
	}
	if relation.Blocked("E", all, closed) || !relation.Blocked("B", all, closed) {
		t.Error("Blocked は BlockedSet と同じ答え")
	}
	if got := relation.BlockedSet(all, func(item.Item) bool { return true }); len(got) != 0 {
		t.Errorf("ブロッカーがすべて終わっていれば誰もブロック中でない: %v", got)
	}
}

func TestBlockedSetAsksClosedOnlyForBlockers(t *testing.T) {
	all := []item.Item{
		{ID: "A", Relations: []item.Relation{rel("blocks", "B")}},
		{ID: "B"},
		{ID: "C", Relations: []item.Relation{rel("relates", "A")}},
	}
	var asked []item.ID
	got := relation.BlockedSet(all, func(it item.Item) bool {
		asked = append(asked, it.ID)
		return false
	})
	if len(got) != 1 || !got["B"] {
		t.Errorf("BlockedSet = %v, want {B}", got)
	}
	if !slices.Equal(asked, []item.ID{"A"}) {
		t.Errorf("closed を呼んだアイテム = %v: blocks を持つ A だけに呼ぶ", asked)
	}
}
