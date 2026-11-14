package transfers_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Review Focus 2: Store S1 の Task を S2 の Epic(TM-1)の下へ、その Epic を S1 へ、同時に移す。Deliver の入口で 2 つの移動を
// 揃え(gate)、さらに、2 つの移動が両方とも移動先の Store のロックを持って fn に入るまで待たせる(inside)。偽の Stores は
// Store ごとの本物のロックを持つので、片方の Store のロックを持ったまま Deliver を呼ぶ実装と、fn が移動元の Store を読む
// 実装は、互いを待って止まる。2 つの移動は止まらずに終わり、Epic の下に Task が入ったので、Epic の移動は移すものが
// 変わったとして失敗して元のままになり、Task の移動は成功する(片方が失敗してもよいが、止まらない)。
func TestOppositeMovesBetweenTwoStoresDoNotStall(t *testing.T) {
	f := newFixture(t)
	f.deliver.gate = newBarrier()   // 2 つの移動が両方 Deliver に入るまで待つ
	f.deliver.inside = newBarrier() // 2 つの移動が両方、移動先のロックを持って fn に入るまで待つ
	toWeb := transfers.MoveInput{ToStore: store1, ToWorkspace: apptest.SampleWorkspace, ToProject: apptest.SampleProject}
	taskDone, epicDone := make(chan error, 1), make(chan error, 1)
	go func() { _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); taskDone <- err }()
	go func() { _, err := f.svc.Move(t.Context(), store2, teamEpic, toWeb); epicDone <- err }()
	timeout := time.After(10 * time.Second)
	var taskErr, epicErr error
	for _, c := range []struct {
		done chan error
		err  *error
	}{{taskDone, &taskErr}, {epicDone, &epicErr}} {
		select {
		case *c.err = <-c.done:
		case <-timeout:
			t.Fatal("2 つの移動が止まった(ロックを入れ子に取っている)")
		}
	}
	if (taskErr == nil) == (epicErr == nil) {
		t.Fatalf("片方だけが成功する: Task の移動 = %v, Epic の移動 = %v", taskErr, epicErr)
	}
	assertMovedOrNot(t, f, apptest.SampleTask, taskErr == nil, store1, store2, teamWS)
	assertMovedOrNot(t, f, teamEpic, epicErr == nil, store2, store1, apptest.SampleWorkspace)
}

// assertMovedOrNot は、id が、moved なら from から消えて to のワークスペース ws にあり、そうでなければ from にだけ元のままあることを確かめる。
func assertMovedOrNot(t *testing.T, f fixture, id item.ID, moved bool, from, to port.StoreID, ws workspace.ID) {
	t.Helper()
	if !moved {
		from, to = to, from
	}
	if exists(t, f.rec, from, id) {
		t.Errorf("%s が %s に残っている(移した = %v)", id, from, moved)
	}
	if it := itemOf(t, f.rec, to, id); moved && it.Place.Workspace != ws {
		t.Errorf("%s = %+v", id, it.Place)
	}
}
