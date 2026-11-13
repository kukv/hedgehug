package items_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func TestMoveUnderAnotherEpicInTheSameProject(t *testing.T) {
	svc, rec := setupRec(t)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")          // HH-3
	first := mustCreate(t, svc, apptest.SampleProject, &epic2.ID, item.Task, "既にある子") // HH-4
	if err := svc.Move(t.Context(), store, apptest.SampleTask, items.MoveInput{Parent: &epic2.ID}); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-2 を HH-3 の下に移動" {
		t.Errorf("summary = %q", got)
	}
	moved := storedItem(t, rec, apptest.SampleTask)
	if moved.Place.Parent == nil || *moved.Place.Parent != epic2.ID || moved.Place.Project != apptest.SampleProject || moved.Place.Number != 2 {
		t.Errorf("place = %+v", moved.Place)
	}
	if moved.Rank <= first.Rank {
		t.Errorf("rank %q は兄弟の末尾(%q のあと)になる", moved.Rank, first.Rank)
	}
	if ops := lastOps(t, rec, apptest.SampleTask, 2); ops[0] != ops[1] {
		t.Errorf("place と rank は同じ操作: %v", ops)
	}
}

func TestMoveIgnoresDeletedSiblingsForTheLastRank(t *testing.T) {
	svc, rec := setupRec(t)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	live := mustCreate(t, svc, apptest.SampleProject, &epic2.ID, item.Task, "残る子")
	gone := mustCreate(t, svc, apptest.SampleProject, &epic2.ID, item.Task, "消した子")
	markDeleted(t, rec, gone.ID, true)
	if err := svc.Move(t.Context(), store, apptest.SampleTask, items.MoveInput{Parent: &epic2.ID}); err != nil {
		t.Fatal(err)
	}
	// 削除済みの順位は比べず、削除されていない兄弟の末尾(残る子のあと)に入る。
	if moved := storedItem(t, rec, apptest.SampleTask); moved.Rank <= live.Rank {
		t.Errorf("rank = %q (live %q)", moved.Rank, live.Rank)
	}
}

func TestMoveAcrossProjectsTakesEveryDescendantIncludingDeleted(t *testing.T) {
	svc, rec := setupRec(t)
	addSecondProject(t, rec)
	epic2 := mustCreate(t, svc, otherProject, nil, item.Epic, "決済")                               // HH-3
	live := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleTask), item.Step, "下書き")  // HH-4
	gone := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleTask), item.Step, "消した子") // HH-5
	markDeleted(t, rec, gone.ID, false)
	addRelation(t, rec, apptest.SampleTask, apptest.SampleEpic, "blocks")
	before := storedItem(t, rec, apptest.SampleTask)
	if err := svc.Move(t.Context(), store, apptest.SampleTask, items.MoveInput{Parent: &epic2.ID}); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-2 を 決済 に移動" {
		t.Errorf("summary = %q", got)
	}
	for _, id := range []item.ID{apptest.SampleTask, live.ID, gone.ID} {
		if it := storedItem(t, rec, id); it.Place.Project != otherProject {
			t.Errorf("%s の project = %s", id, it.Place.Project)
		}
	}
	want := live.Place
	want.Project = otherProject
	if step := storedItem(t, rec, live.ID); !samePlace(step.Place, want) {
		t.Errorf("下のものは親と番号を変えない: %+v", step.Place)
	}
	if gone := storedItem(t, rec, gone.ID); gone.Deleted == nil {
		t.Error("削除済みのまま移る")
	}
	moved := storedItem(t, rec, apptest.SampleTask)
	if !slices.Equal(moved.Relations, before.Relations) || moved.Status != before.Status || moved.Title != before.Title {
		t.Errorf("関連・ステータス・タイトルは変えない: %+v", moved)
	}
	// ツリーは一覧で読み直すので、一覧でも移った先に出て、元のプロジェクトからは消える。
	oldList, err := svc.List(t.Context(), store, apptest.SampleProject)
	if err != nil || len(oldList) != 1 || oldList[0].ID != apptest.SampleEpic {
		t.Errorf("元のプロジェクトの一覧 = %+v, %v", oldList, err)
	}
	newList, err := svc.List(t.Context(), store, otherProject)
	if err != nil || len(newList) != 3 || newList[0].ID != apptest.SampleTask || newList[1].ID != epic2.ID || newList[2].ID != live.ID {
		t.Errorf("移った先の一覧 = %+v, %v", newList, err)
	}
	op := lastOps(t, rec, apptest.SampleTask, 1)[0]
	if lastOps(t, rec, live.ID, 1)[0] != op || lastOps(t, rec, gone.ID, 1)[0] != op {
		t.Error("下のものも同じ操作で移る")
	}
}

// 「親を変更」は、移したものの履歴にだけ出る。一緒に移った下のもの(親は同じ)には出ない。
func TestMoveShowsAParentChangeOnlyOnTheMovedItem(t *testing.T) {
	svc, rec := setupRec(t)
	addSecondProject(t, rec)
	step := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleTask), item.Step, "下書き") // HH-3
	epic4 := mustCreate(t, svc, otherProject, nil, item.Epic, "決済")                              // HH-4
	if err := svc.Move(t.Context(), store, apptest.SampleTask, items.MoveInput{Parent: &epic4.ID}); err != nil {
		t.Fatal(err)
	}
	if got := historyTexts(t, rec, apptest.SampleTask); !slices.Equal(got, []string{"親を変更", "作成"}) {
		t.Errorf("HH-2 の履歴 = %q", got)
	}
	if got := historyTexts(t, rec, step.ID); !slices.Equal(got, []string{"作成"}) {
		t.Errorf("HH-3 の履歴 = %q", got)
	}
}

func TestMoveEpicToAnotherProject(t *testing.T) {
	svc, rec := setupRec(t)
	addSecondProject(t, rec)
	other := mustCreate(t, svc, otherProject, nil, item.Epic, "決済") // HH-3
	if err := svc.Move(t.Context(), store, apptest.SampleEpic, items.MoveInput{Project: pptr(otherProject)}); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-1 を 決済 に移動" {
		t.Errorf("summary = %q", got)
	}
	epic, task := storedItem(t, rec, apptest.SampleEpic), storedItem(t, rec, apptest.SampleTask)
	if epic.Place.Project != otherProject || epic.Place.Parent != nil || epic.Rank <= other.Rank || task.Place.Project != otherProject {
		t.Errorf("epic %+v rank %q (other %q), task %+v", epic.Place, epic.Rank, other.Rank, task.Place)
	}
	if got := historyTexts(t, rec, apptest.SampleEpic); !slices.Equal(got, []string{"親を変更", "作成"}) {
		t.Errorf("HH-1 の履歴 = %q", got)
	}
	if got := historyTexts(t, rec, apptest.SampleTask); !slices.Equal(got, []string{"作成"}) {
		t.Errorf("HH-2 の履歴 = %q", got)
	}
}

func TestMoveRefusesWhatCannotBePlaced(t *testing.T) {
	svc, rec := setupRec(t)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	task2 := mustCreate(t, svc, apptest.SampleProject, &epic2.ID, item.Task, "別の Task")
	step := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleTask), item.Step, "下書き")
	proj := apptest.SampleProject
	cases := []struct {
		name string
		id   item.ID
		in   items.MoveInput
		want string
	}{
		{"Task を Task の下へ", apptest.SampleTask, items.MoveInput{Parent: &task2.ID}, "Task は Task の下に置けません"},
		{"Step を Epic の下へ", step.ID, items.MoveInput{Parent: &epic2.ID}, "Step は Epic の下に置けません"},
		{"Epic を Epic の下へ", epic2.ID, items.MoveInput{Parent: ptr(apptest.SampleEpic)}, "Epic は Epic の下に置けません"},
		{"Epic を自分の子の下へ", apptest.SampleEpic, items.MoveInput{Parent: ptr(apptest.SampleTask)}, "Epic は Task の下に置けません"},
		{"Task を Step の下へ", task2.ID, items.MoveInput{Parent: &step.ID}, "Task は Step の下に置けません"},
		{"Task をプロジェクトの直下へ", apptest.SampleTask, items.MoveInput{Project: &proj}, "プロジェクトの直下には Epic だけを置けます"},
		{"Task を、ないプロジェクトの直下へ", apptest.SampleTask, items.MoveInput{Project: pptr("01NOPE00000000000000000000")}, "プロジェクトの直下には Epic だけを置けます"},
	}
	for _, c := range cases {
		before := logLen(t, rec, c.id)
		err := svc.Move(t.Context(), store, c.id, c.in)
		if message(err) != c.want || logLen(t, rec, c.id) != before {
			t.Errorf("%s: err = %v, log %d → %d", c.name, err, before, logLen(t, rec, c.id))
		}
	}
}

func TestMoveDoesNothingForSelfOrTheSameParent(t *testing.T) {
	svc, rec := setupRec(t)
	proj := apptest.SampleProject
	for name, c := range map[string]struct {
		id item.ID
		in items.MoveInput
	}{
		"自分自身の下":          {apptest.SampleTask, items.MoveInput{Parent: ptr(apptest.SampleTask)}},
		"いまの親":            {apptest.SampleTask, items.MoveInput{Parent: ptr(apptest.SampleEpic)}},
		"Epic のいまのプロジェクト": {apptest.SampleEpic, items.MoveInput{Project: &proj}},
	} {
		before := logLen(t, rec, c.id)
		if err := svc.Move(t.Context(), store, c.id, c.in); err != nil || logLen(t, rec, c.id) != before {
			t.Errorf("%s: err = %v, log %d → %d", name, err, before, logLen(t, rec, c.id))
		}
	}
}

func TestMoveNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	addSecondProject(t, rec)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	trashed := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "削除した Epic")
	markDeleted(t, rec, trashed.ID, true)
	markProjectDeleted(t, rec)
	addOtherWorkspace(t, rec)
	unknown := item.ID("01NOPE00000000000000000000")
	cases := []struct {
		name string
		id   item.ID
		in   items.MoveInput
	}{
		{"ない親", apptest.SampleTask, items.MoveInput{Parent: ptr(unknown)}},
		{"削除済みの親", apptest.SampleTask, items.MoveInput{Parent: &trashed.ID}},
		{"ないアイテム", unknown, items.MoveInput{Parent: &epic2.ID}},
		{"削除済みのアイテム", trashed.ID, items.MoveInput{Parent: &epic2.ID}},
		{"どちらもなし", apptest.SampleTask, items.MoveInput{}},
		{"両方", apptest.SampleTask, items.MoveInput{Parent: &epic2.ID, Project: pptr(otherProject)}},
		{"ないプロジェクト", apptest.SampleEpic, items.MoveInput{Project: pptr("01NOPE00000000000000000000")}},
		{"削除済みのプロジェクト", apptest.SampleEpic, items.MoveInput{Project: pptr(otherProject)}},
		{"別のワークスペースの親", apptest.SampleTask, items.MoveInput{Parent: ptr(mobileEpic)}},
		{"別のワークスペースのプロジェクト", apptest.SampleEpic, items.MoveInput{Project: pptr(mobileProject)}},
	}
	for _, c := range cases {
		before := 0
		if c.id != unknown {
			before = logLen(t, rec, c.id)
		}
		if err := svc.Move(t.Context(), store, c.id, c.in); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: err = %v", c.name, err)
		}
		if c.id != unknown && logLen(t, rec, c.id) != before {
			t.Errorf("%s: 何も保存しない", c.name)
		}
	}
}
