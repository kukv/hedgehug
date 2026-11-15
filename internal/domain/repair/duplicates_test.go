package repair_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// named は、行 row(時刻 t0 の mins 分後)で名前が決まった、値 v の内部 ID id のものを作る。
func named(id, v, row string, mins int) repair.Named {
	return repair.Named{
		Ref:    change.Ref{Kind: change.KindWorkspace, ID: change.ID(id)},
		Field:  "name",
		Value:  v,
		Naming: change.Record{ID: change.ID(row), At: t0.Add(time.Duration(mins) * time.Minute)},
	}
}

func assertRenames(t *testing.T, got []repair.Rename, want map[string]string) {
	t.Helper()
	m := map[string]string{}
	for _, r := range got {
		m[string(r.Ref.ID)] = r.To
	}
	if len(m) != len(want) || len(got) != len(want) {
		t.Fatalf("renames = %v, want %v", m, want)
	}
	for k, v := range want {
		if m[k] != v {
			t.Fatalf("renames = %v, want %v", m, want)
		}
	}
}

func TestNameRenames(t *testing.T) {
	all := []repair.Named{named("a", "開発", "ra", 0), named("b", "開発", "rb", 1)}
	// 名前を決めた行の一方が自分の側 → 自分の側のものが直る。
	assertRenames(t, repair.NameRenames(all, facts([]change.ID{"rb"}, []change.ID{"ra"}), false), map[string]string{"b": "開発 (2)"})
	assertRenames(t, repair.NameRenames(all, facts([]change.ID{"ra"}, []change.ID{"rb"}), false), map[string]string{"a": "開発 (2)"})
	// Facts が空 → at があとの方が直る。
	assertRenames(t, repair.NameRenames(all, repair.Facts{}, false), map[string]string{"b": "開発 (2)"})
	// 重ならなければ空。
	if got := repair.NameRenames([]repair.Named{named("a", "x", "ra", 0), named("b", "y", "rb", 1)}, repair.Facts{}, false); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestNameRenamesSkipTaken(t *testing.T) {
	all := []repair.Named{named("a", "開発", "ra", 0), named("b", "開発", "rb", 1), named("c", "開発 (2)", "rc", 2)}
	assertRenames(t, repair.NameRenames(all, repair.Facts{}, false), map[string]string{"b": "開発 (3)"})

	three := []repair.Named{named("a", "開発", "ra", 0), named("c", "開発", "rc", 2), named("b", "開発", "rb", 1)}
	assertRenames(t, repair.NameRenames(three, repair.Facts{}, false), map[string]string{"b": "開発 (2)", "c": "開発 (3)"})
}

func TestNameRenamesFold(t *testing.T) {
	all := []repair.Named{named("a", "Review", "ra", 0), named("b", "review", "rb", 1)}
	assertRenames(t, repair.NameRenames(all, repair.Facts{}, true), map[string]string{"b": "review (2)"})
	if got := repair.NameRenames(all, repair.Facts{}, false); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	all = append(all, named("c", "Review (2)", "rc", 2))
	assertRenames(t, repair.NameRenames(all, repair.Facts{}, true), map[string]string{"b": "review (3)"})
}

func TestPrefixRenames(t *testing.T) {
	all := []repair.Named{named("a", "WEB", "ra", 0), named("b", "WEB", "rb", 1)}
	assertRenames(t, repair.PrefixRenames(all, repair.Facts{}), map[string]string{"b": "WEB2"})
	all = append(all, named("c", "WEB2", "rc", 2))
	assertRenames(t, repair.PrefixRenames(all, repair.Facts{}), map[string]string{"b": "WEB3"})
}

func TestPrefixRenamesTruncate(t *testing.T) {
	all := []repair.Named{named("a", "ABCDEFGHIJ", "ra", 0), named("b", "ABCDEFGHIJ", "rb", 1)}
	got := repair.PrefixRenames(all, repair.Facts{})
	assertRenames(t, got, map[string]string{"b": "ABCDEFGHI2"})
	if !workspace.ValidPrefix(got[0].To) {
		t.Fatalf("%q は接頭辞の形ではない", got[0].To)
	}
}

func TestKeyRenames(t *testing.T) {
	all := []repair.Named{named("a", "customer", "ra", 0), named("b", "customer", "rb", 1)}
	assertRenames(t, repair.KeyRenames(all, repair.Facts{}), map[string]string{"b": "customer2"})
	all = append(all, named("c", "customer2", "rc", 2))
	assertRenames(t, repair.KeyRenames(all, repair.Facts{}), map[string]string{"b": "customer3"})
}

func setRow(id string, t change.Type, f string, to string, at int, prev ...change.ID) change.Record {
	v, _ := json.Marshal(to)
	return change.Record{ID: change.ID(id), T: t, F: f, To: v, At: t0.Add(time.Duration(at) * time.Minute), Prev: prev}
}

func TestNamingRow(t *testing.T) {
	create := change.Record{ID: "c1", T: change.Create, At: t0}
	set1 := setRow("s1", change.Set, "name", "A", 1)
	set2 := setRow("s2", change.Set, "name", "B", 2, "s1")
	other := setRow("s3", change.Set, "title", "Z", 3)

	got, ok := repair.NamingRow([]change.Record{create, set1, set2, other}, "name")
	if !ok || got.ID != "s2" {
		t.Fatalf("got %v %v, want s2", got.ID, ok)
	}
	got, ok = repair.NamingRow([]change.Record{create, other}, "name")
	if !ok || got.ID != "c1" {
		t.Fatalf("got %v %v, want c1", got.ID, ok)
	}
	if _, ok = repair.NamingRow([]change.Record{other}, "name"); ok {
		t.Fatal("create も set もなければ false")
	}
}
