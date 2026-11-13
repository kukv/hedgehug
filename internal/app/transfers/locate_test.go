package transfers_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const roamer item.ID = "01TESTIT000000000000000020" // 移動を重ねたアイテム

// putRecord は、Store store に転送記録を直接足す(Entity は item)。
func putRecord(t *testing.T, f fixture, store port.StoreID, target item.ID, old, newID string, to port.StoreID, toName string) {
	t.Helper()
	write(t, f.rec, store, func(tx port.Tx) error {
		return tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: target, Old: old, New: newID, ToStore: string(to), ToStoreName: toName})
	})
}

func locate(t *testing.T, f fixture, store, display string) transfers.Located {
	t.Helper()
	got, err := f.svc.Locate(t.Context(), store, display)
	if err != nil {
		t.Fatalf("Locate(%q, %q) = %v", store, display, err)
	}
	return got
}

// いまの ID。同じ ID が 2 つの Store にあるときは、Store 名の Store を先に、なければ並びの先頭の Store。
func TestLocateCurrentIDPrefersTheNamedStore(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f.rec, store2, workspace.Workspace{ID: "01TESTWS000000000000000020", Name: "Web2", Prefix: "HH"}, "01TESTPR000000000000000020", "01TESTIT000000000000000021", nil)
	other := addItem(t, f.rec, store2, item.Item{ID: "01TESTIT000000000000000022", Place: item.Place{Workspace: "01TESTWS000000000000000020", Project: "01TESTPR000000000000000020", Parent: ptr("01TESTIT000000000000000021"), Number: 2}, Kind: item.Task, Rank: "V", Title: "別の HH-2"})
	if got := locate(t, f, "team", "HH-2"); !got.Found || got.Store != store2 || got.Item != other.ID || got.Project != "01TESTPR000000000000000020" || got.MovedFrom != nil || got.Gone != nil {
		t.Errorf("team: %+v", got)
	}
	for _, name := range []string{"main", "zzz"} { // 名前の Store になければ並びの先頭
		if got := locate(t, f, name, "HH-2"); !got.Found || got.Store != store1 || got.Item != apptest.SampleTask || got.Project != apptest.SampleProject {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

// 旧い接頭辞(HH → WEB)で見つかる。あとから別のワークスペース(Mobile)が HH を使うと、いまの接頭辞の方を開く。
func TestLocateOldPrefix(t *testing.T) {
	f := newFixture(t)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "WEB"))
	})
	if got := locate(t, f, "main", "HH-2"); !got.Found || got.Item != apptest.SampleTask || got.MovedFrom != nil {
		t.Errorf("旧い接頭辞: %+v", got)
	}
	if got := locate(t, f, "main", "WEB-2"); got.Item != apptest.SampleTask {
		t.Errorf("いまの接頭辞: %+v", got)
	}
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(mobileWS, change.NewSet("prefix", "MB", "HH"))
	})
	mobileTask := addItem(t, f.rec, store1, item.Item{ID: "01TESTIT000000000000000023", Place: item.Place{Workspace: mobileWS, Project: mobileProject, Parent: ptr(mobileEpic), Number: 2}, Kind: item.Task, Rank: "V", Title: "Mobile の 2"})
	if got := locate(t, f, "main", "HH-2"); got.Item != mobileTask.ID {
		t.Errorf("いまの接頭辞のワークスペースが優先: %+v", got)
	}
}

// Review Focus 3: A → B → A(番号は再利用しないので、戻ったあとの ID は HH-3)。旧 ID はどちらも、いまの場所に辿り着く。
func TestLocateOfAnItemMovedAwayAndBack(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	back := transfers.MoveInput{ToStore: store1, ToWorkspace: apptest.SampleWorkspace, ToProject: apptest.SampleProject, ToParent: ptr(apptest.SampleEpic)}
	if got, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, back); err != nil || got.Display != "HH-3" {
		t.Fatalf("戻す: %+v, %v", got, err)
	}
	recs := recordsOf(t, f.rec, store1) // HH-2 → MB-2、MB-2 → HH-3
	first := locate(t, f, "main", "HH-2")
	if !first.Found || first.Item != apptest.SampleTask || first.Project != apptest.SampleProject || first.MovedFrom == nil ||
		first.MovedFrom.Old != "HH-2" || first.MovedFrom.Display != "MB-2" || first.MovedFrom.Current != "HH-3" || !first.MovedFrom.At.Equal(recs[0].At) {
		t.Errorf("HH-2: %+v, %+v", first, first.MovedFrom)
	}
	if got := locate(t, f, "main", "MB-2"); got.Item != apptest.SampleTask || got.MovedFrom == nil || got.MovedFrom.Display != "HH-3" || got.MovedFrom.Current != "HH-3" || !got.MovedFrom.At.Equal(recs[1].At) {
		t.Errorf("MB-2: %+v", got)
	}
	if got := locate(t, f, "main", "HH-3"); got.Item != apptest.SampleTask || got.MovedFrom != nil {
		t.Errorf("HH-3(いまの ID): %+v", got)
	}
}

// 1 回だけ移したとき、記録の新 ID(Display)といまの ID(Current)は同じ。画面は「…へ移動しました」の 1 文にする。
func TestLocateAfterASingleMoveHasTheSameDisplayAndCurrent(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	got := locate(t, f, "main", "HH-2")
	if got.MovedFrom == nil || got.MovedFrom.Old != "HH-2" || got.MovedFrom.Display != "MB-2" || got.MovedFrom.Current != "MB-2" {
		t.Errorf("HH-2: %+v", got.MovedFrom)
	}
}

// Review Focus 3: A → B → C。A の記録(HH-9 → TM-2)はそのまま、いまの場所(閲覧のみの Store の Archive)へ辿り着く。
// 閲覧のみの Store の中でも、表示だけは解決できる。HH-2 はサンプルの Task が使っているので、旧 ID は使っていない HH-9。
func TestLocateFollowsAChainAcrossStores(t *testing.T) {
	f := newFixture(t)
	addItem(t, f.rec, store3, item.Item{ID: roamer, Place: item.Place{Workspace: archiveWS, Project: archiveProj, Parent: ptr(archiveEpic), Number: 7}, Kind: item.Task, Rank: "V", Title: "旅"})
	putRecord(t, f, store1, roamer, "HH-9", "TM-2", store2, "team")
	putRecord(t, f, store2, roamer, "TM-2", "AR-7", store3, "readonly")
	got := locate(t, f, "main", "HH-9")
	if !got.Found || got.Store != store3 || got.Item != roamer || got.Project != archiveProj || got.MovedFrom == nil || got.MovedFrom.Old != "HH-9" || got.MovedFrom.Display != "TM-2" || got.MovedFrom.Current != "AR-7" {
		t.Errorf("HH-9: %+v", got)
	}
	if got := locate(t, f, "main", "TM-2"); got.Store != store3 || got.MovedFrom == nil || got.MovedFrom.Display != "AR-7" || got.MovedFrom.Current != "AR-7" || !got.MovedFrom.At.Equal(recordsOf(t, f.rec, store2)[0].At) {
		t.Errorf("TM-2: %+v", got)
	}
}

// Review Focus 3: 同じものを 2 人が別々に移して、記録が 2 件(TM-2 と AR-7)。内部 ID の現在地(team)に辿り着く。
// 添える日時と記録の新 ID(Display)は、最初に一致した行(新しい方の記録)のもの。いまの ID(Current)はアイテムのいる team の TM-2。
func TestLocateWithTwoRecordsOfTheSameItem(t *testing.T) {
	f := newFixture(t)
	addItem(t, f.rec, store2, item.Item{ID: roamer, Place: item.Place{Workspace: teamWS, Project: teamProject, Parent: ptr(teamEpic), Number: 2}, Kind: item.Task, Rank: "V", Title: "旅"})
	putRecord(t, f, store1, roamer, "HH-9", "TM-2", store2, "team")
	putRecord(t, f, store1, roamer, "HH-9", "AR-7", store3, "readonly")
	recs := recordsOf(t, f.rec, store1)
	if got := locate(t, f, "main", "HH-9"); got.Store != store2 || got.Item != roamer || got.MovedFrom == nil || got.MovedFrom.Display != "AR-7" || got.MovedFrom.Current != "TM-2" || !got.MovedFrom.At.Equal(recs[1].At) {
		t.Errorf("HH-9: %+v", got)
	}
}

// 見つからないとき(画面は開かず、4 つの文を言い分ける)。ここでは理由と材料だけを確かめる(文は Task 11)。
// 記録の日時は、偽の `Stores` の時計(2026-11-12 09:00 ごろ)。
func TestLocateGoneReasons(t *testing.T) {
	const gone item.ID = "01TESTIT000000000000000030"
	nineAM := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		setup func(t *testing.T, f fixture)
		want  transfers.Gone
	}{
		{"削除済み", func(t *testing.T, f fixture) {
			if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
				t.Fatal(err)
			}
			if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
				t.Fatal(err)
			}
		}, transfers.Gone{Reason: "deleted", Old: "HH-2", At: nineAM, NewDisplay: "MB-2"}},
		{"完全に削除", func(t *testing.T, f fixture) {
			putRecord(t, f, store1, gone, "HH-2", "TM-2", store2, "team")
			f.rec.AddPurged(store2, port.Purged{Entity: "item", Target: gone, Display: "TM-2", At: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)})
		}, transfers.Gone{Reason: "purged", Old: "HH-2", At: nineAM, NewDisplay: "TM-2"}},
		{"移動先の Store が表示中でない", func(t *testing.T, f fixture) {
			putRecord(t, f, store1, gone, "HH-2", "WEB-31", "S9", "elsewhere")
		}, transfers.Gone{Reason: "hidden", Old: "HH-2", At: nineAM, NewDisplay: "WEB-31", StoreName: "elsewhere"}},
		{"より新しい記録の移動先の Store が表示中でない", func(t *testing.T, f fixture) {
			putRecord(t, f, store1, gone, "HH-2", "WEB-31", store2, "team")
			putRecord(t, f, store2, gone, "WEB-31", "TM-5", "S9", "elsewhere")
		}, transfers.Gone{Reason: "elsewhere", Old: "HH-2", At: nineAM, NewDisplay: "WEB-31", StoreName: "elsewhere"}},
	}
	for _, c := range cases {
		f := newFixture(t)
		if c.name != "削除済み" { // サンプルの Task が HH-2 を使っているので、削除して旧 ID を空ける
			if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
				t.Fatal(err)
			}
		}
		c.setup(t, f)
		got, err := f.svc.Locate(t.Context(), "main", "HH-2")
		checkGone(t, c.name, got, err, c.want, recordsOf(t, f.rec, store1)[0].At) // どの場面も、HH-2 の記録は main の 1 件
	}
}

// checkGone は、Gone が want と同じかを確かめる。日時は、旧 ID が一致した記録の日時 at(09:00 ごろ)。
func checkGone(t *testing.T, name string, got transfers.Located, err error, want transfers.Gone, at time.Time) {
	t.Helper()
	if err != nil || got.Found || got.Gone == nil {
		t.Errorf("%s: %+v, %v", name, got, err)
		return
	}
	g := *got.Gone
	if !g.At.Equal(at) || !at.Truncate(time.Minute).Equal(want.At) {
		t.Errorf("%s: 日時 = %v, want %v(旧 ID が一致した記録の日時)", name, g.At, at)
	}
	g.At = want.At
	if g != want {
		t.Errorf("%s: Gone = %+v, want %+v", name, g, want)
	}
}

// {Store 名} は、移動先の Store がこのマシンにあれば(非表示でも)いまの名前で、記録の ToStoreName ではない。
// このマシンにない Store は記録の ToStoreName(TestLocateGoneReasons の S9)。
func TestLocateGoneStoreNameIsTheNameOnThisMachine(t *testing.T) {
	f := newFixture(t)
	const gone item.ID = "01TESTIT000000000000000030"
	putRecord(t, f, store1, gone, "HH-9", "WEB-31", store2, "team")
	putRecord(t, f, store2, gone, "WEB-31", "AR-5", store3, "古い名前")
	f.list.Stores = f.list.Stores[:2] // readonly を非表示にする
	got, err := f.svc.Locate(t.Context(), "main", "HH-9")
	if err != nil || got.Gone == nil || got.Gone.Reason != "elsewhere" || got.Gone.StoreName != "readonly" {
		t.Errorf("非表示の Store: %+v, %v", got, err)
	}
}

// 理由がなければ見つからない(ホームへ)。削除しただけのいまの ID も同じ。
func TestLocateNotFound(t *testing.T) {
	f := newFixture(t)
	for _, id := range []string{"ZZ-9", "nonsense", "HH-", "-3", "HH-+2", "HH-2x", "HH-9"} {
		if _, err := f.svc.Locate(t.Context(), "main", id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%q: %v", id, err)
		}
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Locate(t.Context(), "main", "HH-2"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除しただけのいまの ID: %v", err)
	}
}

// OldIDs は、旧い接頭辞と転送記録で旧 ID に当たる、削除されていないアイテムをすべて返す(いまの ID は含めない)。
// 同じ Store の 2 つのワークスペースが同じ旧い接頭辞を持てば、両方を返す(ワークスペースの並び)。
func TestOldIDs(t *testing.T) {
	f := newFixture(t)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return errors.Join( // Web(HH → WEB)と Mobile(MB → HH → MX)の両方が、旧い接頭辞に HH を持つ
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "WEB")),
			tx.WriteWorkspaces().Append(mobileWS, change.NewSet("prefix", "MB", "HH")),
			tx.WriteWorkspaces().Append(mobileWS, change.NewSet("prefix", "HH", "MX")),
		)
	})
	mobileTask := addItem(t, f.rec, store1, item.Item{ID: "01TESTIT000000000000000023", Place: item.Place{Workspace: mobileWS, Project: mobileProject, Parent: ptr(mobileEpic), Number: 2}, Kind: item.Task, Rank: "V", Title: "Mobile の 2"})
	addItem(t, f.rec, store3, item.Item{ID: roamer, Place: item.Place{Workspace: archiveWS, Project: archiveProj, Parent: ptr(archiveEpic), Number: 7}, Kind: item.Task, Rank: "V", Title: "旅"})
	putRecord(t, f, store2, roamer, "HH-2", "AR-7", store3, "readonly")
	putRecord(t, f, store2, "01TESTIT000000000000000030", "HH-2", "TM-9", store2, "team") // 移した先にない
	got, err := f.svc.OldIDs(t.Context(), "HH-2")
	want := []transfers.OldID{{Store: store1, Item: apptest.SampleTask}, {Store: store1, Item: mobileTask.ID}, {Store: store3, Item: roamer}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("OldIDs(HH-2) = %+v, %v, want %+v", got, err, want)
	}
	if got, err := f.svc.OldIDs(t.Context(), "WEB-2"); err != nil || len(got) != 0 {
		t.Errorf("いまの ID: %+v, %v", got, err)
	}
}

// teamHH は、team(S2)に接頭辞 HH のワークスペース Web2 と、その HH-2(main の HH-2 とは無関係なアイテム)を足す。
func teamHH(t *testing.T, f fixture) item.Item {
	t.Helper()
	addWorkspace(t, f.rec, store2, workspace.Workspace{ID: "01TESTWS000000000000000020", Name: "Web2", Prefix: "HH"}, "01TESTPR000000000000000020", "01TESTIT000000000000000021", nil)
	return addItem(t, f.rec, store2, item.Item{ID: "01TESTIT000000000000000022", Place: item.Place{Workspace: "01TESTWS000000000000000020", Project: "01TESTPR000000000000000020", Parent: ptr("01TESTIT000000000000000021"), Number: 2}, Kind: item.Task, Rank: "V", Title: "別の HH-2"})
}

// Store 名の Store の旧 ID(転送記録)は、ほかの Store のいまの ID より先に選ぶ。main の HH-2 を移したあと、
// /main の HH-2 は移したアイテムを開き、team の無関係な HH-2 を開かない。/team の HH-2 は team のいまの ID。
// 別の Store への移動と、同じ Store の中の移動のどちらでも同じ。
func TestLocateOldIDOfTheNamedStoreBeatsACurrentIDOfAnotherStore(t *testing.T) {
	for _, to := range []transfers.MoveInput{toTeam(), toMobile()} {
		f := newFixture(t)
		other := teamHH(t, f)
		moved, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, to)
		if err != nil {
			t.Fatal(err)
		}
		got := locate(t, f, "main", "HH-2")
		if !got.Found || got.Store != to.ToStore || got.Item != apptest.SampleTask || got.MovedFrom == nil || got.MovedFrom.Display != moved.Display {
			t.Errorf("main の HH-2(%s へ): %+v", moved.Display, got)
		}
		if got := locate(t, f, "team", "HH-2"); got.Item != other.ID || got.MovedFrom != nil {
			t.Errorf("team の HH-2: %+v", got)
		}
	}
}

// Store 名の Store の転送記録で開けない(移した先で削除済み)ときも、ほかの Store の同じ ID を開かず、理由を返す。
func TestLocateGoneInTheNamedStoreBeatsACurrentIDOfAnotherStore(t *testing.T) {
	f := newFixture(t)
	teamHH(t, f)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if got := locate(t, f, "main", "HH-2"); got.Found || got.Gone == nil || got.Gone.Reason != "deleted" {
		t.Errorf("main の HH-2: %+v", got)
	}
}

// 接頭辞を変えた(HH → WEB)あとに移したアイテム(記録の旧 ID は WEB-2)は、変える前の ID(HH-2)からも辿れる。
// 移したあとにさらに接頭辞を変えても(WEB → WEB2)、HH-2・WEB-2 のどちらからも辿れる。
func TestLocateFollowsARecordWrittenAfterThePrefixChanged(t *testing.T) {
	f := newFixture(t)
	setPrefix := func(from, to string) {
		write(t, f.rec, store1, func(tx port.Tx) error {
			return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", from, to))
		})
	}
	setPrefix("HH", "WEB")
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	setPrefix("WEB", "WEB2")
	for _, id := range []string{"HH-2", "WEB-2"} {
		got := locate(t, f, "main", id)
		if !got.Found || got.Item != apptest.SampleTask || got.MovedFrom == nil || got.MovedFrom.Old != "WEB-2" || got.MovedFrom.Display != "MB-2" {
			t.Errorf("%s: %+v", id, got)
		}
	}
}
