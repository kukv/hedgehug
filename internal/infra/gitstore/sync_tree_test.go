package gitstore

import (
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// line は、log.jsonl の 1 行を作る。
func line(t *testing.T, r change.Record) string {
	t.Helper()
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

func at(minute int) time.Time { return time.Date(2026, 11, 12, 10, minute, 0, 0, time.UTC) }

func itemCreate(t *testing.T, id string, parent *item.ID, when time.Time) string {
	t.Helper()
	it := item.Item{ID: item.ID(id), Place: item.Place{Workspace: "W1", Project: "P1", Parent: parent, Number: 1}, Kind: item.Task, Rank: "a", Title: "t"}
	if parent == nil {
		it.Kind = item.Epic
	}
	d := item.Create(it)
	return line(t, change.Record{ID: change.ID("c" + id), Op: "o", At: when, By: "taro", Actor: change.User("taro"), T: d.T, To: d.To})
}

func setTitle(t *testing.T, recID, to string, when time.Time, by string, prev ...change.ID) string {
	t.Helper()
	d := change.NewSet("title", "t", to)
	return line(t, change.Record{ID: change.ID(recID), Op: change.ID(recID), At: when, By: by, Actor: change.User(by), T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev})
}

func mustMerge(t *testing.T, base, a, b tree) tree {
	t.Helper()
	got, err := mergeTrees(base, a, b)
	if err != nil {
		t.Fatal(err)
	}
	swapped, err := mergeTrees(base, b, a)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.EqualFunc(got, swapped, func(x, y []byte) bool { return string(x) == string(y) }) {
		t.Fatalf("the result depends on which side merges:\n%v\n%v", keys(got), keys(swapped))
	}
	return got
}

func keys(t tree) []string { return sortedKeys(t) }

func TestMergeTreesLaterTitleWinsAndYAMLIsRebuilt(t *testing.T) {
	create := itemCreate(t, "I1", nil, at(0))
	base := tree{"items/I1/log.jsonl": []byte(create)}
	a := tree{"items/I1/log.jsonl": []byte(create + setTitle(t, "rA", "A", at(1), "ann", "cI1"))}
	b := tree{"items/I1/log.jsonl": []byte(create + setTitle(t, "rB", "B", at(2), "bob", "cI1"))}
	got := mustMerge(t, base, a, b)
	if n := strings.Count(string(got["items/I1/log.jsonl"]), "\n"); n != 3 {
		t.Errorf("log has %d lines, want 3", n)
	}
	if y := string(got["items/I1/item.yaml"]); !strings.Contains(y, "title: B\n") {
		t.Errorf("item.yaml =\n%s", y)
	}
}

func TestMergeTreesDeletedFolderWins(t *testing.T) {
	create := itemCreate(t, "I1", nil, at(0))
	base := tree{"items/I1/log.jsonl": []byte(create), "items/I1/item.yaml": []byte("x")}
	a := tree{} // 完全に削除した
	b := tree{"items/I1/log.jsonl": []byte(create + setTitle(t, "rB", "B", at(2), "bob", "cI1"))}
	got := mustMerge(t, base, a, b)
	for p := range got {
		if strings.HasPrefix(p, "items/I1/") {
			t.Errorf("%s is left", p)
		}
	}
}

func TestMergeTreesNewFolderOnOneSideIsKept(t *testing.T) {
	a := tree{"items/I2/log.jsonl": []byte(itemCreate(t, "I2", nil, at(0)))}
	got := mustMerge(t, tree{}, a, tree{})
	if _, ok := got["items/I2/item.yaml"]; !ok {
		t.Errorf("new item lost: %v", keys(got))
	}
}

func TestMergeTreesDropsChildrenOfTransferred(t *testing.T) {
	// a は Epic E1 を別の Store へ移した(フォルダを消し、転送記録を足した)。b は同時に E1 の下に Task T1 を作り、
	// T1 の下に Step S1 を作った。T1 と S1 は反映しない(06「別の Store へ移動したものと重なったとき」)。
	epic := itemCreate(t, "E1", nil, at(0))
	e1 := item.ID("E1")
	t1 := item.ID("T1")
	base := tree{"items/E1/log.jsonl": []byte(epic)}
	a := tree{"transfers.jsonl": []byte(`{"id":"X1","op":"X1","at":"2026-11-12T10:05:00.000Z","by":"taro","target":"E1","old":"HH-1","new":"WEB-31","to_store":"S2","to_store_name":"team-b","to_remote":""}` + "\n")}
	b := tree{
		"items/E1/log.jsonl": []byte(epic),
		"items/T1/log.jsonl": []byte(itemCreate(t, "T1", &e1, at(3))),
		"items/S1/log.jsonl": []byte(itemCreate(t, "S1", &t1, at(4))),
	}
	got := mustMerge(t, base, a, b)
	for p := range got {
		if strings.HasPrefix(p, "items/") {
			t.Errorf("%s is left", p)
		}
	}
	if _, ok := got["transfers.jsonl"]; !ok {
		t.Error("transfers.jsonl lost")
	}
}

func TestMergeTreesSameStoreMoveKeepsChildren(t *testing.T) {
	// 同じ Store の別のワークスペースへの移動も転送記録を残すが、フォルダは残るので、子は消さない。
	epic := itemCreate(t, "E1", nil, at(0))
	e1 := item.ID("E1")
	transfer := []byte(`{"id":"X1","op":"X1","at":"2026-11-12T10:05:00.000Z","by":"taro","target":"E1","old":"HH-1","new":"WEB-31"}` + "\n")
	base := tree{"items/E1/log.jsonl": []byte(epic)}
	a := tree{"items/E1/log.jsonl": []byte(epic), "transfers.jsonl": transfer}
	b := tree{"items/E1/log.jsonl": []byte(epic), "items/T1/log.jsonl": []byte(itemCreate(t, "T1", &e1, at(3)))}
	if got := mustMerge(t, base, a, b); got["items/T1/log.jsonl"] == nil {
		t.Error("child of an item moved within the store was dropped")
	}
}

func TestMergeTreesDescription(t *testing.T) {
	desc := func(id string, when time.Time) string {
		return line(t, change.Record{ID: change.ID(id), Op: change.ID(id), At: when, By: "x", Actor: change.User("x"), T: "desc"})
	}
	create := itemCreate(t, "I1", nil, at(0))
	base := tree{"items/I1/log.jsonl": []byte(create), "items/I1/description.md": []byte("a\nb\n")}
	a := tree{"items/I1/log.jsonl": []byte(create + desc("dA", at(9))), "items/I1/description.md": []byte("a\nNEW-A\n")}
	b := tree{"items/I1/log.jsonl": []byte(create + desc("dB", at(5))), "items/I1/description.md": []byte("a\nNEW-B\n")}
	got := mustMerge(t, base, a, b)
	// b の保存のほうが古いので先に置く。
	if d := string(got["items/I1/description.md"]); d != "a\nNEW-B\n-----\nNEW-A\n" {
		t.Errorf("description = %q", d)
	}
}

func TestMergeTreesUnknownFiles(t *testing.T) {
	base := tree{"README.md": []byte("base")}
	if got := mustMerge(t, base, tree{"README.md": []byte("base")}, tree{"README.md": []byte("edited")}); string(got["README.md"]) != "edited" {
		t.Errorf("one side's edit lost: %q", got["README.md"])
	}
	if got := mustMerge(t, base, tree{"README.md": []byte("aaa")}, tree{"README.md": []byte("zzz")}); string(got["README.md"]) != "zzz" {
		t.Errorf("both edited: %q", got["README.md"])
	}
	if got := mustMerge(t, base, tree{}, tree{"README.md": []byte("base")}); got["README.md"] != nil {
		t.Errorf("one side's removal lost")
	}
}
