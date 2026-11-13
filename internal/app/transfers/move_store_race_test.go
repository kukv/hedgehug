package transfers_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/transfers"
)

// Review Focus 2: Store S1 の Task を S2 へ、S2 の Epic(TM-1)を S1 へ、同時に移す。Deliver の入口で 2 つの移動を揃え(gate)、
// さらに、2 つの移動が両方とも移動先の Store のロックを持って fn に入るまで待たせる(inside)。偽の Stores は Store ごとの
// 本物のロックを持つので、片方の Store のロックを持ったまま Deliver を呼ぶ実装と、fn が移動元の Store を読む実装は、
// 互いを待って止まる。
func TestOppositeMovesBetweenTwoStoresDoNotStall(t *testing.T) {
	f := newFixture(t)
	f.deliver.gate = newBarrier(2)   // 2 つの移動が両方 Deliver に入るまで待つ
	f.deliver.inside = newBarrier(2) // 2 つの移動が両方、移動先のロックを持って fn に入るまで待つ
	toWeb := transfers.MoveInput{ToStore: store1, ToWorkspace: apptest.SampleWorkspace, ToProject: apptest.SampleProject}
	done := make(chan error, 2)
	go func() { _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); done <- err }()
	go func() { _, err := f.svc.Move(t.Context(), store2, teamEpic, toWeb); done <- err }()
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("移動が失敗した: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("2 つの移動が止まった(ロックを入れ子に取っている)")
		}
	}
	if it := itemOf(t, f.rec, store2, apptest.SampleTask); it.Place.Workspace != teamWS {
		t.Errorf("Task = %+v", it.Place)
	}
	if it := itemOf(t, f.rec, store1, teamEpic); it.Place.Workspace != apptest.SampleWorkspace {
		t.Errorf("Epic = %+v", it.Place)
	}
}
