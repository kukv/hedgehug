package transfers_test

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func toMobile() transfers.MoveInput {
	return transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject, ToParent: ptr(mobileEpic)}
}

const (
	stepGoneID item.ID = "01TESTIT000000000000000010"
	stepLiveID item.ID = "01TESTIT000000000000000011"
)

// Task HH-2 の下に、削除済みの Step(HH-3)→生きている Step(HH-4)の順に作る。
func withSteps(t *testing.T, f fixture) {
	t.Helper()
	parent := apptest.SampleTask
	for i, id := range []item.ID{stepGoneID, stepLiveID} {
		addItem(t, f.rec, store1, item.Item{ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 3 + i}, Kind: item.Step, Rank: "V", Title: "s"})
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, stepGoneID); err != nil {
		t.Fatal(err)
	}
}

func TestMoveToAnotherWorkspaceRenumbersDeletedChildrenAfterLiveOnes(t *testing.T) {
	f := newFixture(t)
	withSteps(t, f)
	deletedAt := itemOf(t, f.rec, store1, stepGoneID).Deleted.At
	got, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile())
	if err != nil {
		t.Fatal(err)
	}
	if got.Display != "MB-2" || got.Item != apptest.SampleTask || got.ToStore != store1 || len(got.Emptied) != 0 {
		t.Errorf("moved = %+v", got)
	}
	// 移すアイテムが MB-2、生きている子が MB-3、削除済みの子が MB-4(作った順では逆)。
	for id, want := range map[item.ID]int{apptest.SampleTask: 2, stepLiveID: 3, stepGoneID: 4} {
		it := itemOf(t, f.rec, store1, id)
		if it.Place.Workspace != mobileWS || it.Place.Project != mobileProject || it.Place.Number != want {
			t.Errorf("%s の場所 = %+v, 番号は %d", id, it.Place, want)
		}
	}
	task := itemOf(t, f.rec, store1, apptest.SampleTask)
	if task.Place.Parent == nil || *task.Place.Parent != mobileEpic || task.Rank != rank.First {
		t.Errorf("移すアイテムの親と順位 = %v, %q(Mobile の Epic の子の末尾)", task.Place.Parent, task.Rank)
	}
	if step := itemOf(t, f.rec, store1, stepLiveID); *step.Place.Parent != apptest.SampleTask || step.Rank != "V" {
		t.Errorf("下のものの親と順位は変えない: %+v", step.Place)
	}
	// 削除済みのまま、元の削除日時。
	if gone := itemOf(t, f.rec, store1, stepGoneID); gone.Deleted == nil || !gone.Deleted.At.Equal(deletedAt) {
		t.Errorf("削除済みの印 = %+v", gone.Deleted)
	}
	// 番号は移動先のワークスペースの次の番号から使い切る。移動元の番号は戻らない。
	if n := nextNumber(t, f.rec, store1, mobileWS); n != 5 {
		t.Errorf("Mobile の次の番号 = %d, want 5", n)
	}
	if n := nextNumber(t, f.rec, store1, apptest.SampleWorkspace); n != 5 {
		t.Errorf("Web の次の番号 = %d(再利用しない), want 5", n)
	}
}

// 1 回の保存、転送記録 3 件(アイテムごと)、アクティビティ。
func TestMoveWithinStoreIsOneSaveWithTransferRecords(t *testing.T) {
	f := newFixture(t)
	withSteps(t, f)
	before := len(f.rec.summaries())
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if got := f.rec.summaries()[before:]; !slices.Equal(got, []string{"HH-2 を Web から Mobile へ移動"}) {
		t.Errorf("書いたアクティビティ = %q", got)
	}
	recs := recordsOf(t, f.rec, store1)
	byTarget := map[item.ID]string{}
	for _, r := range recs {
		byTarget[r.Target] = r.Old + ">" + r.New
		if r.Entity != "item" || r.ToStore != string(store1) || r.ToStoreName != "" || r.ToRemote != "" || r.By != "hanako" {
			t.Errorf("記録 = %+v", r)
		}
	}
	want := map[item.ID]string{apptest.SampleTask: "HH-2>MB-2", stepLiveID: "HH-4>MB-3", stepGoneID: "HH-3>MB-4"}
	if len(recs) != 3 || !maps.Equal(byTarget, want) {
		t.Errorf("転送記録 = %v", byTarget)
	}
	// 移すアイテムの place の行と記録は同じ操作。
	log := logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	var placeOp change.ID
	for _, r := range log {
		if r.T == change.Set && r.F == "place" {
			placeOp = r.Op
		}
	}
	for _, r := range recs {
		if r.Op != placeOp {
			t.Errorf("記録の操作 %s と place の操作 %s が違う", r.Op, placeOp)
		}
	}
}

// Epic は、プロジェクトの直下(親なし)に移り、下の Task もついて行く。
func TestMoveEpicIntoAnotherWorkspaceTakesItsTasks(t *testing.T) {
	f := newFixture(t)
	in := transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject}
	got, err := f.svc.Move(t.Context(), store1, apptest.SampleEpic, in)
	if err != nil || got.Display != "MB-2" {
		t.Fatalf("moved = %+v, %v", got, err)
	}
	epic, task := itemOf(t, f.rec, store1, apptest.SampleEpic), itemOf(t, f.rec, store1, apptest.SampleTask)
	if epic.Place.Parent != nil || epic.Place.Number != 2 || epic.Rank <= "V" || task.Place.Number != 3 || *task.Place.Parent != apptest.SampleEpic || task.Place.Workspace != mobileWS {
		t.Errorf("epic = %+v, task = %+v", epic, task)
	}
}

func TestMoveRemapsValuesAndReportsEmptiedOnes(t *testing.T) {
	f := newFixtureWith(t, func(st *workspace.Settings) {
		withoutPending(st)
		st.Priorities[2].Name = "Normal" // Medium が合わなくなる。初期優先度はこの値
	})
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask,
			change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusPending),
			change.NewSet("assignee", nil, "hanako"))
	})
	inProgress := valueID(t, f.rec, "status", "InProgress")
	in := toMobile()
	in.StatusChoices = map[change.ID]change.ID{apptest.SampleStatusPending: inProgress}
	got, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, in)
	if err != nil {
		t.Fatal(err)
	}
	it := itemOf(t, f.rec, store1, apptest.SampleTask)
	if it.Status != inProgress {
		t.Errorf("ステータスは選んだ置き換え: %q", it.Status)
	}
	if it.Priority != valueID(t, f.rec, "priority", "Normal") || !slices.Equal(got.Emptied, []string{"優先度"}) {
		t.Errorf("優先度 = %q, emptied = %v(初期優先度にして「優先度」を並べる)", it.Priority, got.Emptied)
	}
	if it.Assignee != "hanako" || !slices.Equal(it.Watchers, []string{"taro"}) {
		t.Errorf("同じ Store の人はそのまま: %q, %v", it.Assignee, it.Watchers)
	}
}

// 置き換えを渡さないと、同じ分類(Pending は未着手)の移動先の既定(Todo)。
func TestMoveWithoutAStatusChoiceUsesTheDefaultOfTheSameCategory(t *testing.T) {
	f := newFixtureWith(t, withoutPending)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusPending))
	})
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if it := itemOf(t, f.rec, store1, apptest.SampleTask); it.Status != valueID(t, f.rec, "status", "Todo") {
		t.Errorf("status = %q", it.Status)
	}
}

func TestMoveRefusesWhatCannotBeChosen(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	same := transfers.MoveInput{ToStore: store1, ToWorkspace: apptest.SampleWorkspace, ToProject: apptest.SampleProject, ToParent: ptr(apptest.SampleEpic)}
	cases := []struct {
		name string
		id   item.ID
		in   transfers.MoveInput
		want error
	}{
		{"同じワークスペース", apptest.SampleTask, same, port.ErrNotFound},
		{"Task に親がない", apptest.SampleTask, transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject}, port.ErrNotFound},
		{"Epic に親がある", apptest.SampleEpic, toMobile(), port.ErrNotFound},
		{"プロジェクトが別のワークスペースのもの", apptest.SampleTask, transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: apptest.SampleProject, ToParent: ptr(mobileEpic)}, port.ErrNotFound},
		{"親が移動先のプロジェクトの Epic でない", apptest.SampleTask, transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject, ToParent: ptr(apptest.SampleEpic)}, port.ErrNotFound},
		{"閲覧のみの Store", apptest.SampleTask, transfers.MoveInput{ToStore: store3, ToWorkspace: archiveWS, ToProject: archiveProj, ToParent: ptr(archiveEpic)}, port.ErrNotFound},
		{"表示中でない Store", apptest.SampleTask, transfers.MoveInput{ToStore: "S9", ToWorkspace: mobileWS, ToProject: mobileProject, ToParent: ptr(mobileEpic)}, port.ErrStoreNotFound},
	}
	for _, c := range cases {
		before := len(f.rec.summaries())
		if _, err := f.svc.Move(ctx, store1, c.id, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
		if len(f.rec.summaries()) != before {
			t.Errorf("%s: 何も保存しない", c.name)
		}
	}
	f.list.Stores[0].ReadOnly = true // 移動元が閲覧のみ
	var r *refusal.Refusal
	if _, err := f.svc.Move(ctx, store1, apptest.SampleTask, toMobile()); !errors.As(err, &r) || r.Message != "閲覧のみの Store です" {
		t.Errorf("移動元が閲覧のみ: %v", err)
	}
}

// 移すアイテムは、移動先の親の削除されていない子の末尾に入る。削除済みの子(順位が後ろの X)は数えない。
func TestMovePlacesTheItemAfterTheLastLiveChildOfTheNewParent(t *testing.T) {
	f := newFixture(t)
	parent := mobileEpic
	mk := func(id item.ID, n int, r string) item.Item {
		return addItem(t, f.rec, store1, item.Item{ID: id, Place: item.Place{Workspace: mobileWS, Project: mobileProject, Parent: &parent, Number: n}, Kind: item.Task, Rank: r, Title: "t"})
	}
	mk("01TESTIT000000000000000020", 2, "V")
	gone := mk("01TESTIT000000000000000021", 3, "X")
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, gone.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if got, want := itemOf(t, f.rec, store1, apptest.SampleTask).Rank, rank.After("V"); got != want {
		t.Errorf("rank = %q, want %q(生きている子 V のあと)", got, want)
	}
}

// 順位が変わらない(移動先の親に子がなく、いまも rank.First)ときは、rank の行を書かない。
func TestMoveDoesNotWriteAnUnchangedRank(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	for _, r := range logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}) {
		if r.T == change.Set && r.F == "rank" {
			t.Errorf("rank の行を書いた: %+v", r)
		}
	}
}
