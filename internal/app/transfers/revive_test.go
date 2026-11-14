package transfers_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// deletedElsewhere は、withSteps のあと、別の画面で Task HH-2 を削除した状態にする(生きている Step HH-4 は同じ操作で
// 一緒に削除。HH-3 はその前に別の操作で削除済み)。
func deletedElsewhere(t *testing.T, f fixture) {
	t.Helper()
	withSteps(t, f)
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask, stepLiveID); err != nil {
		t.Fatal(err)
	}
}

// wantRevivedMove は、移したあと、Task と同じ操作で削除された Step HH-4 は削除済みでなく、前に別の操作で削除した
// Step HH-3 は削除済みのまま移っていること、取り消したのは移動前の ID(HH-2)であることを確かめる。
func wantRevivedMove(t *testing.T, f fixture, store port.StoreID, revived []string) {
	t.Helper()
	for _, c := range []struct {
		id      item.ID
		deleted bool
	}{{apptest.SampleTask, false}, {stepLiveID, false}, {stepGoneID, true}} {
		if got := itemOf(t, f.rec, store, c.id); (got.Deleted != nil) != c.deleted {
			t.Errorf("%s: Deleted = %+v, want deleted %v", c.id, got.Deleted, c.deleted)
		}
	}
	if !slices.Equal(revived, []string{"HH-2"}) {
		t.Errorf("revived = %v, want [HH-2]", revived)
	}
}

// 06「他の書き手との重なり」: 別の画面で削除されたアイテムを移すと、同じワークスペースの中の移動と同じく、削除を取り消して移す。
func TestMoveWithinStoreRevivesAnItemDeletedElsewhere(t *testing.T) {
	f := newFixture(t)
	deletedElsewhere(t, f)
	ctx := port.WithRevived(t.Context())
	if _, err := f.svc.Move(ctx, store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if got := itemOf(t, f.rec, store1, apptest.SampleTask); got.Place.Workspace != mobileWS {
		t.Errorf("place = %+v", got.Place)
	}
	wantRevivedMove(t, f, store1, port.Revived(ctx))
}

// 別の Store へも同じ。写した行の削除の行を、移動先の同じ保存で取り消す。移動元はフォルダを消す。
func TestMoveAcrossStoresRevivesAnItemDeletedElsewhere(t *testing.T) {
	f := newFixture(t)
	deletedElsewhere(t, f)
	ctx := port.WithRevived(t.Context())
	if _, err := f.svc.Move(ctx, store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	if exists(t, f.rec, store1, apptest.SampleTask) {
		t.Error("the source folder is still there")
	}
	wantRevivedMove(t, f, store2, port.Revived(ctx))
}

// 親と一緒に削除されたもの(root: false)は、同じワークスペースの中の移動と同じく、移せない(ErrNotFound)。
func TestMoveRefusesAnItemDeletedWithItsParent(t *testing.T) {
	f := newFixture(t)
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	for _, in := range []transfers.MoveInput{toMobile(), toTeam()} {
		if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, in); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Move to %s = %v, want ErrNotFound", in.ToStore, err)
		}
	}
}

// 別の画面で削除されたアイテムを別の Store へ移すと、同じ削除の操作で消えた下のもの(順位が前の Step)は、削除を取り消す
// ので、削除されていない Step と一緒に並び順で番号を振る(同じ Store の移動と同じ)。
func TestMoveAcrossStoresNumbersRevivedChildrenInOrder(t *testing.T) {
	f := newFixture(t)
	const first, second item.ID = "01TESTIT000000000000000014", "01TESTIT000000000000000015"
	parent := apptest.SampleTask
	for i, s := range []struct {
		id   item.ID
		rank string
	}{{first, "A"}, {second, "V"}} {
		addItem(t, f.rec, store1, item.Item{ID: s.id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 3 + i}, Kind: item.Step, Rank: s.rank, Title: "s"})
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask, first); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Move(port.WithRevived(t.Context()), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	for id, n := range map[item.ID]int{apptest.SampleTask: 2, first: 3, second: 4} {
		if it := itemOf(t, f.rec, store2, id); it.Place.Number != n || it.Deleted != nil {
			t.Errorf("%s number = %d (deleted %v), want %d", id, it.Place.Number, it.Deleted != nil, n)
		}
	}
}
