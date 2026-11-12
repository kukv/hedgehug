package gitstore

import (
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
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

func TestMergeTreesDescriptionTieOrdersByBody(t *testing.T) {
	// 最後の desc の at が同じなら、本文のバイト順で古い版を決める。
	desc := func(id string) string {
		return line(t, change.Record{ID: change.ID(id), Op: change.ID(id), At: at(9), By: "x", Actor: change.User("x"), T: "desc"})
	}
	create := itemCreate(t, "I1", nil, at(0))
	base := tree{"items/I1/log.jsonl": []byte(create), "items/I1/description.md": []byte("a\nb\n")}
	a := tree{"items/I1/log.jsonl": []byte(create + desc("dA")), "items/I1/description.md": []byte("a\nNEW-B\n")}
	b := tree{"items/I1/log.jsonl": []byte(create + desc("dB")), "items/I1/description.md": []byte("a\nNEW-A\n")}
	got := mustMerge(t, base, a, b)
	if d := string(got["items/I1/description.md"]); d != "a\nNEW-A\n-----\nNEW-B\n" {
		t.Errorf("description = %q", d)
	}
}

func TestMergeTreesDescriptionGoneOnBothSidesStaysGone(t *testing.T) {
	l := []byte(itemCreate(t, "I1", nil, at(0)))
	base := tree{"items/I1/log.jsonl": l, "items/I1/description.md": []byte("a\n")}
	got := mustMerge(t, base, tree{"items/I1/log.jsonl": l}, tree{"items/I1/log.jsonl": l})
	if _, ok := got["items/I1/description.md"]; ok {
		t.Errorf("description.md was recreated: %q", got["items/I1/description.md"])
	}
}

func TestMergeTreesUnknownFileRemovedOnOneSideEditedOnOther(t *testing.T) {
	base := tree{"README.md": []byte("base")}
	got := mustMerge(t, base, tree{}, tree{"README.md": []byte("edited")})
	if string(got["README.md"]) != "edited" {
		t.Errorf("edit lost: %q", got["README.md"])
	}
}

func transferLine(target string) []byte {
	return []byte(`{"id":"X1","op":"X1","at":"2026-11-12T10:05:00.000Z","by":"taro","target":"` + target + `","old":"HH-1","new":"WEB-31","to_store":"S2","to_store_name":"team-b","to_remote":""}` + "\n")
}

func TestMergeTreesDropsWholeFolderOfChildOfTransferred(t *testing.T) {
	epic := itemCreate(t, "E1", nil, at(0))
	e1 := item.ID("E1")
	base := tree{"items/E1/log.jsonl": []byte(epic)}
	a := tree{"transfers.jsonl": transferLine("E1")}
	b := tree{
		"items/E1/log.jsonl":        []byte(epic),
		"items/T1/log.jsonl":        []byte(itemCreate(t, "T1", &e1, at(3))),
		"items/T1/description.md":   []byte("body\n"),
		"items/T1/item.yaml":        []byte("x"),
		"items/T1/attachments/a.md": []byte("x"),
	}
	for p := range mustMerge(t, base, a, b) {
		if strings.HasPrefix(p, "items/") {
			t.Errorf("%s is left", p)
		}
	}
}

func rec(t *testing.T, id string, kind change.Type, d change.Draft, when time.Time) string {
	t.Helper()
	return line(t, change.Record{ID: change.ID(id), Op: change.ID(id), At: when, By: "taro", Actor: change.User("taro"), T: kind, F: d.F, From: d.From, To: d.To})
}

func workspaceCreate(t *testing.T) string {
	t.Helper()
	d, err := workspace.Create(workspace.Workspace{ID: "W1", Name: "Hedgehug", Prefix: "HH"})
	if err != nil {
		t.Fatal(err)
	}
	return rec(t, "w0", change.Create, d, at(0))
}

func projectCreate(t *testing.T) string {
	t.Helper()
	p := project.Project{ID: "P1", Place: project.Place{Workspace: "W1", Number: 1}, Rank: "a", Title: "Old", Status: "active"}
	return rec(t, "p0", change.Create, project.Create(p), at(0))
}

func TestMergeTreesRebuildsMembersWorkspaceProjectYAML(t *testing.T) {
	init := rec(t, "m0", change.Create, member.Init(), at(0))
	ws, pr := workspaceCreate(t), projectCreate(t)
	title := func(id, to string, when time.Time) string {
		return rec(t, id, change.Set, change.NewSet("title", "Old", to), when)
	}
	base := tree{"members/log.jsonl": []byte(init), "workspaces/W1/log.jsonl": []byte(ws), "projects/P1/log.jsonl": []byte(pr)}
	a := tree{
		"members/log.jsonl":       []byte(init + rec(t, "m1", change.Add, member.Join("ann"), at(1))),
		"workspaces/W1/log.jsonl": []byte(ws),
		"projects/P1/log.jsonl":   []byte(pr + title("pA", "Alpha", at(1))),
	}
	b := tree{
		"members/log.jsonl":       []byte(init + rec(t, "m2", change.Add, member.Join("bob"), at(2))),
		"workspaces/W1/log.jsonl": []byte(ws),
		"projects/P1/log.jsonl":   []byte(pr + title("pB", "Beta", at(2))),
	}
	got := mustMerge(t, base, a, b)
	if y := string(got["members/members.yaml"]); !strings.Contains(y, "login: ann") || !strings.Contains(y, "login: bob") {
		t.Errorf("members.yaml =\n%s", y)
	}
	if y := string(got["workspaces/W1/workspace.yaml"]); !strings.Contains(y, "prefix: HH") {
		t.Errorf("workspace.yaml =\n%s", y)
	}
	if y := string(got["projects/P1/project.yaml"]); !strings.Contains(y, "title: Beta") {
		t.Errorf("project.yaml =\n%s", y)
	}
}

func TestMergeTreesTransferOfProjectOrWorkspaceDropsChildren(t *testing.T) {
	ws, pr := workspaceCreate(t), projectCreate(t)
	base := tree{"workspaces/W1/log.jsonl": []byte(ws), "projects/P1/log.jsonl": []byte(pr)}
	newSide := func() tree {
		return tree{
			"workspaces/W1/log.jsonl": []byte(ws),
			"projects/P1/log.jsonl":   []byte(pr),
			"items/I1/log.jsonl":      []byte(itemCreate(t, "I1", nil, at(3))),
		}
	}
	// プロジェクトを移した: そのプロジェクトの下のアイテムが消え、ワークスペースは残る。
	got := mustMerge(t, base, tree{"workspaces/W1/log.jsonl": []byte(ws), "transfers.jsonl": transferLine("P1")}, newSide())
	if got["items/I1/log.jsonl"] != nil || got["projects/P1/log.jsonl"] != nil || got["workspaces/W1/log.jsonl"] == nil {
		t.Errorf("project transfer: %v", keys(got))
	}
	// ワークスペースを移した: その下のプロジェクトとアイテムが消える。
	got = mustMerge(t, base, tree{"transfers.jsonl": transferLine("W1")}, newSide())
	if got["items/I1/log.jsonl"] != nil || got["projects/P1/log.jsonl"] != nil || got["workspaces/W1/log.jsonl"] != nil {
		t.Errorf("workspace transfer: %v", keys(got))
	}
}
