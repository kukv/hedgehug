package syncing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

func newAuto(t *testing.T) (*fixture, *syncing.Auto, *apptest.SyncSettings) {
	t.Helper()
	f := newFixture(t)
	settings := &apptest.SyncSettings{Value: port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second}}
	return f, syncing.NewAuto(f.svc, settings), settings
}

func step(t *testing.T, a *syncing.Auto) {
	t.Helper()
	if err := a.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// 06「自動で同期する」: 開いた直後に、対象(remote・表示中・アカウントあり)のすべての Store。
// 閲覧のみも対象。間隔が過ぎるまでは、もう一度は同期しない。
func TestAutoSyncsTargetsWhenOpened(t *testing.T) {
	f, a, _ := newAuto(t)
	step(t, a)
	n := f.syncs()
	if n["S1"] != 1 || n["S4"] != 1 || n["S2"] != 0 || n["S3"] != 0 || n["S5"] != 0 {
		t.Fatalf("syncs = %v", n)
	}
	f.clock.Advance(4 * time.Minute)
	step(t, a)
	if n := f.syncs(); n["S1"] != 1 {
		t.Errorf("synced before the interval: %v", n)
	}
	f.clock.Advance(time.Minute)
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 || n["S4"] != 2 {
		t.Errorf("not synced at the interval: %v", n)
	}
}

// 「書き込み後」: 最後の保存から 30 秒、次の保存がなければ、その Store。
func TestAutoSyncsAfterWritesSettle(t *testing.T) {
	f, a, _ := newAuto(t)
	step(t, a)
	f.clock.Advance(time.Minute)
	a.Wrote("S1")
	f.clock.Advance(20 * time.Second)
	a.Wrote("S1") // 続けて保存すると、最後の保存から数え直す
	f.clock.Advance(20 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 1 {
		t.Fatalf("synced while writes continued: %d", n)
	}
	f.clock.Advance(10 * time.Second)
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 || n["S4"] != 1 {
		t.Fatalf("after quiet: %v", n)
	}
	f.clock.Advance(40 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 2 {
		t.Errorf("synced again without a write: %d", n)
	}
}

// 同期エラーの Store は、書き込み後でも、最後に試みてから間隔が過ぎるまで待つ。
func TestAutoErrorStoreWaitsForTheInterval(t *testing.T) {
	f, a, _ := newAuto(t)
	f.rep.Errs["S1"] = port.ErrAuthFailed
	step(t, a)
	f.clock.Advance(2 * time.Minute)
	a.Wrote("S1")
	f.clock.Advance(31 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 1 {
		t.Fatalf("error store synced after a write: %d", n)
	}
	f.clock.Advance(3 * time.Minute)
	step(t, a)
	if n := f.syncs()["S1"]; n != 2 {
		t.Errorf("not synced at the interval: %d", n)
	}
}

// 見えているタブが待つ条件にあるあいだは始めず、解けたら続けて同期する。知らせは 30 秒で切れる。
func TestAutoWaitsWhileATabHolds(t *testing.T) {
	f, a, _ := newAuto(t)
	a.Hold("tab-1", true)
	step(t, a)
	if len(f.rep.Calls()) != 0 {
		t.Fatal("synced while held")
	}
	a.Hold("tab-1", false)
	step(t, a)
	if n := f.syncs()["S1"]; n != 1 {
		t.Fatalf("not synced after release: %d", n)
	}
	a.Network("tab-2", false)
	a.Network("tab-2", true) // オンライン復帰の契機
	a.Hold("tab-2", true)
	f.clock.Advance(31 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 2 {
		t.Errorf("an expired hold still blocks: %d", n)
	}
}

// 決定 C2-18b: オフラインのあいだは、どの契機でも試さない(エラーを記録しない)。つながったら、間隔を待たずに、
// 対象のすべての Store。オフラインの知らせは 2 分で切れる(決定 S2)。
func TestAutoWaitsWhileOfflineAndSyncsWhenBackOnline(t *testing.T) {
	f, a, _ := newAuto(t)
	a.Network("tab-1", false)
	step(t, a)
	f.clock.Advance(20 * time.Second)
	a.Network("tab-1", false) // 送り直し
	a.Wrote("S1")
	f.clock.Advance(time.Minute - 20*time.Second)
	a.Network("tab-1", false)
	step(t, a)
	if len(f.rep.Calls()) != 0 {
		t.Fatalf("synced while offline: %+v", f.rep.Calls())
	}
	if rs, _ := f.state.Records(context.Background()); len(rs) != 0 {
		t.Fatalf("recorded while offline: %+v", rs)
	}
	a.Network("tab-1", true)
	step(t, a)
	if n := f.syncs(); n["S1"] != 1 || n["S4"] != 1 {
		t.Fatalf("after back online: %v", n)
	}
	f.clock.Advance(time.Minute)
	a.Network("tab-1", true) // オフラインでなかったタブの online は契機にしない
	step(t, a)
	if n := f.syncs(); n["S1"] != 1 {
		t.Errorf("online without offline triggered: %v", n)
	}
	a.Wrote("S1")
	a.Network("tab-2", false)
	f.clock.Advance(time.Minute + 30*time.Second) // 見えないタブの間引き(1 分に 1 回)より長く持つ
	step(t, a)
	if n := f.syncs(); n["S1"] != 1 {
		t.Fatalf("offline expired too early: %v", n)
	}
	f.clock.Advance(31 * time.Second)
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 {
		t.Errorf("expired offline still blocks: %v", n) // 書き込み後の契機(S1)で同期する
	}
}

// 1 つの Store の同期が生のエラーで返っても、同じ Step の残りの Store は同期し、Step はエラーを返す。
func TestAutoContinuesAfterAStoreError(t *testing.T) {
	f, a, _ := newAuto(t)
	f.tokens.Errs = map[port.StoreID]error{"S1": errors.New("keychain broken")}
	if err := a.Step(context.Background()); err == nil {
		t.Fatal("Step = nil, want the S1 error")
	}
	if n := f.syncs(); n["S4"] != 1 {
		t.Errorf("S4 starved by S1's error: %v", n)
	}
}

// 開いたとき・オンライン復帰の契機は、間隔の「まだ試していない」とは別。記録ができたあとでも、間隔を待たずに同期する。
func TestAutoAllTriggerSurvivesAHold(t *testing.T) {
	f, a, _ := newAuto(t)
	step(t, a) // 記録を作る
	f.clock.Advance(time.Minute)
	a.Network("tab-2", false)
	a.Network("tab-2", true) // オンライン復帰の契機
	a.Hold("tab-1", true)
	step(t, a)
	if n := f.syncs()["S1"]; n != 1 {
		t.Fatalf("synced while held: %d", n)
	}
	a.Hold("tab-1", false)
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 || n["S4"] != 2 {
		t.Errorf("all trigger lost during the hold: %v", n)
	}
}

func TestAutoBackOnlineSyncsAllBeforeTheInterval(t *testing.T) {
	f, a, _ := newAuto(t)
	step(t, a) // 記録を作る
	f.clock.Advance(time.Minute)
	a.Network("tab-1", false)
	f.clock.Advance(20 * time.Second)
	a.Network("tab-1", true)
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 || n["S4"] != 2 {
		t.Errorf("back online did not sync all: %v", n)
	}
}

// 同期が動いているあいだの保存は、同期で送ったものとして消さず、あとの書き込み後の契機で同期する。
func TestAutoWriteDuringASyncSurvives(t *testing.T) {
	f, a, _ := newAuto(t)
	step(t, a)
	f.clock.Advance(time.Minute)
	a.Wrote("S1")
	f.clock.Advance(30 * time.Second)
	f.rep.Gate = make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() { done <- a.Step(context.Background()) }()
	for f.syncs()["S1"] < 2 { // Sync が始まって止まるまで待つ
		time.Sleep(time.Millisecond)
	}
	f.clock.Advance(5 * time.Second)
	a.Wrote("S1")
	f.rep.Gate <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	f.rep.Gate = nil
	f.clock.Advance(30 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 3 {
		t.Errorf("write during the sync was dropped: %d", n)
	}
}

// 「裏で自動的に同期する」がオフなら、どの契機でも同期しない。
func TestAutoOffStopsEverything(t *testing.T) {
	f, a, settings := newAuto(t)
	settings.Value.Auto = false
	a.Wrote("S1")
	a.Network("tab-1", false)
	a.Network("tab-1", true)
	f.clock.Advance(time.Hour)
	step(t, a)
	if len(f.rep.Calls()) != 0 {
		t.Errorf("synced while off: %+v", f.rep.Calls())
	}
}
