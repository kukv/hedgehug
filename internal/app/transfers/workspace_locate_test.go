package transfers_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 古いアドレスの解決のテストで足す Store。S4「other」はサイドバーの最後、S5「zzz」は main のすぐ後ろ(team より前)。
const (
	store4 port.StoreID = "S4"
	store5 port.StoreID = "S5"

	zzzWS      workspace.ID = "01TESTWS000000000000000050"
	zzzProject project.ID   = "01TESTPR000000000000000050"
	zzzEpic    item.ID      = "01TESTIT000000000000000050"
	goneWS     workspace.ID = "01TESTWS000000000000000051" // どの Store にもないワークスペース
)

// addStore は、Store id(名前 name)を足し、サイドバーの at 番目に置く。このマシンの Store(SyncTargets)にも足す。
func addStore(t *testing.T, f fixture, id port.StoreID, name string, at int) {
	t.Helper()
	f.rec.AddStore(id)
	addMembers(t, f.rec, id)
	f.list.Stores = slices.Insert(f.list.Stores, at, port.StoreInfo{ID: id, Name: name})
	ts := f.svc.SyncTargets.(*apptest.SyncTargets)
	ts.Targets = append(ts.Targets, port.SyncTarget{ID: id, Name: name})
}

// withZZZ は、サイドバーで team より前に Store「zzz」を置き、そこに接頭辞 HH のワークスペース(HH-P1・HH-1 を持つ)を作る。
func withZZZ(t *testing.T, f fixture) {
	t.Helper()
	addStore(t, f, store5, "zzz", 1)
	addWorkspace(t, f.rec, store5, workspace.Workspace{ID: zzzWS, Name: "Z", Prefix: "HH"}, zzzProject, zzzEpic, nil)
}

func moveWeb(t *testing.T, f fixture, from, to port.StoreID) {
	t.Helper()
	if _, err := f.svc.MoveWorkspace(t.Context(), from, apptest.SampleWorkspace, to); err != nil {
		t.Fatal(err)
	}
}

// wsRecord は、Store store のワークスペースの転送記録のうち、最後のもの。
func wsRecord(t *testing.T, f fixture, store port.StoreID) transfer.Record {
	t.Helper()
	recs := recordsOf(t, f.rec, store)
	i := slices.IndexFunc(recs, func(r transfer.Record) bool { return r.Entity == transfer.EntityWorkspace })
	if i < 0 {
		t.Fatalf("%s にワークスペースの記録がない: %+v", store, recs)
	}
	return recs[i]
}

func locateWorkspace(t *testing.T, f fixture, store, prefix string) transfers.LocatedWorkspace {
	t.Helper()
	got, err := f.svc.LocateWorkspace(t.Context(), store, prefix)
	if err != nil {
		t.Fatalf("LocateWorkspace(%q, %q) = %v", store, prefix, err)
	}
	return got
}

// B5c-13: 移したワークスペースは、移動元の古いアドレスから移動先のものを開き、MovedFrom の Display は記録の移動先の Store 名、
// Current はいまの Store 名。さらに別の Store へ移すと、Display は最初の移動先のまま、Current はいまの Store。
// 普通に開いたときは MovedFrom を付けない。
func TestLocateWorkspaceMoved(t *testing.T) {
	f := newFixture(t)
	addStore(t, f, store4, "other", 3)
	if got := locateWorkspace(t, f, "main", "MB"); !got.Found || got.Store != store1 || got.Workspace != mobileWS || got.MovedFrom != nil || got.Gone != nil {
		t.Errorf("いまの接頭辞: %+v", got)
	}
	moveWeb(t, f, store1, store2)
	at := wsRecord(t, f, store1).At
	got := locateWorkspace(t, f, "main", "HH")
	want := transfers.MovedFrom{Old: "HH", At: at, Display: "team", Current: "team"}
	if !got.Found || got.Store != store2 || got.Workspace != apptest.SampleWorkspace || got.MovedFrom == nil || *got.MovedFrom != want {
		t.Errorf("team へ移したあと: %+v, %+v", got, got.MovedFrom)
	}
	if got := locateWorkspace(t, f, "team", "HH"); got.Store != store2 || got.MovedFrom != nil {
		t.Errorf("いまの場所: %+v", got)
	}
	moveWeb(t, f, store2, store4)
	got = locateWorkspace(t, f, "main", "HH")
	want.Current = "other"
	if !got.Found || got.Store != store4 || got.Workspace != apptest.SampleWorkspace || got.MovedFrom == nil || *got.MovedFrom != want {
		t.Errorf("other へ移したあと: %+v, %+v", got, got.MovedFrom)
	}
	got = locateWorkspace(t, f, "team", "HH")
	if want := (transfers.MovedFrom{Old: "HH", At: wsRecord(t, f, store2).At, Display: "other", Current: "other"}); got.MovedFrom == nil || *got.MovedFrom != want {
		t.Errorf("team の古いアドレス: %+v", got.MovedFrom)
	}
}

// Review Focus 4: Store 名の Store の転送記録を、ほかの Store のいまの接頭辞より先に見る。zzz の古いアドレスは zzz のもの。
func TestLocateWorkspaceNamedRecordFirst(t *testing.T) {
	f := newFixture(t)
	withZZZ(t, f)
	moveWeb(t, f, store1, store2)
	if got := locateWorkspace(t, f, "main", "HH"); got.Store != store2 || got.Workspace != apptest.SampleWorkspace || got.MovedFrom == nil {
		t.Errorf("main: %+v", got)
	}
	if got := locateWorkspace(t, f, "zzz", "HH"); got.Store != store5 || got.Workspace != zzzWS || got.MovedFrom != nil {
		t.Errorf("zzz: %+v", got)
	}
	if got := locateWorkspace(t, f, "nope", "HH"); got.Store != store5 || got.Workspace != zzzWS || got.MovedFrom != nil {
		t.Errorf("ない Store 名は、ほかの Store のいまの接頭辞から: %+v", got)
	}
}

// B5c-13: 接頭辞を HH から WEB に変えてから移すと記録の old は WEB。変える前の HH でも、見つけたワークスペースの旧い接頭辞で
// 記録に当て、zzz のいまの HH より先に開く(MovedFrom.Old は画面の接頭辞 HH)。中のアイテム・プロジェクトも同じ。
// 移動先の Store が非表示なら、旧い接頭辞は表示中の Store でしか引かないので記録に当たらず、zzz のいまの HH-1 を開く。
func TestLocateWorkspaceRenamedBeforeMove(t *testing.T) {
	f := newFixture(t)
	withZZZ(t, f)
	setWebPrefix(t, f, "HH", "WEB")
	moveWeb(t, f, store1, store2)
	got := locateWorkspace(t, f, "main", "HH")
	if !got.Found || got.Store != store2 || got.Workspace != apptest.SampleWorkspace || got.MovedFrom == nil || got.MovedFrom.Old != "HH" || got.MovedFrom.Display != "team" {
		t.Errorf("LocateWorkspace(HH): %+v, %+v", got, got.MovedFrom)
	}
	if got, err := f.svc.Locate(t.Context(), "main", "HH-1"); err != nil || got.Store != store2 || got.Item != apptest.SampleEpic || got.MovedFrom != nil {
		t.Errorf("Locate(HH-1): %+v, %v", got, err)
	}
	if got := locateProject(t, f, "main", "HH-P1"); got.Store != store2 || got.Project != apptest.SampleProject || got.MovedFrom != nil {
		t.Errorf("LocateProject(HH-P1): %+v", got)
	}
	if got := locateWorkspace(t, f, "main", "WEB"); got.MovedFrom == nil || got.MovedFrom.Old != "WEB" {
		t.Errorf("LocateWorkspace(WEB): %+v", got.MovedFrom)
	}
	f.list.Stores = slices.DeleteFunc(f.list.Stores, func(s port.StoreInfo) bool { return s.ID == store2 })
	if got, err := f.svc.Locate(t.Context(), "main", "HH-1"); err != nil || !got.Found || got.Store != store5 || got.Item != zzzEpic {
		t.Errorf("移動先が非表示の Locate(HH-1): %+v, %v", got, err)
	}
}

// アイテムの種類の記録は、ワークスペースの記録より細かい。Store 名の Store で、HH-1 の種類の記録が一致して開けない(完全に
// 削除)なら、ワークスペースの記録で移動先の HH-1 を開く前に、その理由を返す。
func TestLocateItemRecordBeforeWorkspaceRecord(t *testing.T) {
	f := newFixture(t)
	moveWeb(t, f, store1, store2)
	const gone item.ID = "01TESTIT000000000000000051"
	putRecord(t, f, store1, gone, "HH-1", "TM-9", store2, "team")
	f.rec.AddPurged(store2, port.Purged{Entity: transfer.EntityItem, Target: gone, Display: "TM-9", At: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)})
	got, err := f.svc.Locate(t.Context(), "main", "HH-1")
	if err != nil || got.Found || got.Gone == nil || got.Gone.Reason != "purged" || got.Gone.Old != "HH-1" || got.Gone.NewDisplay != "TM-9" {
		t.Errorf("Locate(HH-1): %+v, %+v, %v", got, got.Gone, err)
	}
}

// B5c-13: 開けないときの Gone は A5a-15 の 4 つ。Old は画面の接頭辞、NewDisplay は記録の移動先の Store のこのマシンの名前。
func TestLocateWorkspaceGone(t *testing.T) {
	hide := func(f fixture, id port.StoreID) {
		f.list.Stores = slices.DeleteFunc(f.list.Stores, func(s port.StoreInfo) bool { return s.ID == id })
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, f fixture)
		want  transfers.Gone
	}{
		{"移動先の Store が非表示", func(t *testing.T, f fixture) {
			moveWeb(t, f, store1, store2)
			hide(f, store2)
		}, transfers.Gone{Reason: "hidden", Old: "HH", NewDisplay: "team", StoreName: "team"}},
		{"移動先で削除", func(t *testing.T, f fixture) {
			moveWeb(t, f, store1, store2)
			if err := apptest.DeleteWorkspace(t.Context(), f.rec, store2, apptest.SampleWorkspace); err != nil {
				t.Fatal(err)
			}
		}, transfers.Gone{Reason: "deleted", Old: "HH", NewDisplay: "team"}},
		{"そのあと非表示の Store へ移した", func(t *testing.T, f fixture) {
			addStore(t, f, store4, "other", 3)
			moveWeb(t, f, store1, store2)
			moveWeb(t, f, store2, store4)
			hide(f, store4)
		}, transfers.Gone{Reason: "elsewhere", Old: "HH", NewDisplay: "team", StoreName: "other"}},
		{"完全に削除", func(t *testing.T, f fixture) {
			write(t, f.rec, store1, func(tx port.Tx) error {
				return tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityWorkspace, Target: goneWS, Old: "GW", New: "GW", ToStore: string(store2), ToStoreName: "古い名前"})
			})
			f.rec.AddPurged(store2, port.Purged{Entity: transfer.EntityWorkspace, Target: goneWS, Display: "GW", At: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)})
		}, transfers.Gone{Reason: "purged", Old: "GW", NewDisplay: "team"}},
	}
	for _, c := range cases {
		f := newFixture(t)
		c.setup(t, f)
		got, err := f.svc.LocateWorkspace(t.Context(), "main", c.want.Old)
		if err != nil || got.Found || got.Gone == nil {
			t.Errorf("%s: %+v, %v", c.name, got, err)
			continue
		}
		if at := wsRecord(t, f, store1).At; !got.Gone.At.Equal(at) {
			t.Errorf("%s: 日時 = %v, want %v", c.name, got.Gone.At, at)
		}
		g := *got.Gone
		g.At = time.Time{}
		if g != c.want {
			t.Errorf("%s: Gone = %+v, want %+v", c.name, g, c.want)
		}
	}
}

// 削除済みのワークスペースは、いまの接頭辞でも開かない。読めない接頭辞・どこにもない接頭辞は port.ErrNotFound。
func TestLocateWorkspaceNotFound(t *testing.T) {
	f := newFixture(t)
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store1, mobileWS); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"MB", "NOPE", ""} {
		if got, err := f.svc.LocateWorkspace(t.Context(), "main", prefix); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%q: %+v, %v", prefix, got, err)
		}
	}
}

// B5c-13: 移したワークスペースの中のアイテム・プロジェクトは、移動元の古いアドレスから、ワークスペースの記録で移動先の同じ番号の
// ものを開く(zzz のいまの HH-1 より先。ID は変わらないので MovedFrom はない)。開けないときの Gone は、Old が画面の ID、
// NewDisplay が記録の移動先の Store 名。移動先でそのアイテムだけ削除されていても deleted。
func TestLocateItemViaWorkspaceRecord(t *testing.T) {
	f := newFixture(t)
	withZZZ(t, f)
	moveWeb(t, f, store1, store2)
	if got, err := f.svc.Locate(t.Context(), "main", "HH-1"); err != nil || !got.Found || got.Store != store2 || got.Item != apptest.SampleEpic || got.MovedFrom != nil {
		t.Errorf("Locate(HH-1): %+v, %v", got, err)
	}
	if got := locateProject(t, f, "main", "HH-P1"); !got.Found || got.Store != store2 || got.Project != apptest.SampleProject || got.MovedFrom != nil {
		t.Errorf("LocateProject(HH-P1): %+v", got)
	}
	if ids, err := f.svc.OldIDs(t.Context(), "HH-1"); err != nil || len(ids) != 0 {
		t.Errorf("OldIDs はいまの ID で見つかるものを含めない: %+v, %v", ids, err)
	}
	at := wsRecord(t, f, store1).At
	if err := apptest.DeleteItems(t.Context(), f.rec, store2, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Locate(t.Context(), "main", "HH-2")
	checkWsGone(t, "移動先でアイテムを削除", got.Gone, err, transfers.Gone{Reason: "deleted", Old: "HH-2", At: at, NewDisplay: "team"})
	shownStores := f.list.Stores
	f.list.Stores = slices.DeleteFunc(slices.Clone(shownStores), func(s port.StoreInfo) bool { return s.ID == store2 })
	got, err = f.svc.Locate(t.Context(), "main", "HH-1")
	checkWsGone(t, "非表示(アイテム)", got.Gone, err, transfers.Gone{Reason: "hidden", Old: "HH-1", At: at, NewDisplay: "team", StoreName: "team"})
	p, err := f.svc.LocateProject(t.Context(), "main", "HH-P1")
	checkWsGone(t, "非表示(プロジェクト)", p.Gone, err, transfers.Gone{Reason: "hidden", Old: "HH-P1", At: at, NewDisplay: "team", StoreName: "team"})
	f.list.Stores = shownStores
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store2, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	got, err = f.svc.Locate(t.Context(), "main", "HH-1")
	checkWsGone(t, "移動先でワークスペースを削除", got.Gone, err, transfers.Gone{Reason: "deleted", Old: "HH-1", At: at, NewDisplay: "team"})
}

func checkWsGone(t *testing.T, name string, got *transfers.Gone, err error, want transfers.Gone) {
	t.Helper()
	if err != nil || got == nil {
		t.Errorf("%s: %+v, %v", name, got, err)
		return
	}
	if *got != want {
		t.Errorf("%s: Gone = %+v, want %+v", name, *got, want)
	}
}

// ほかの Store を探すときも、種類の記録の理由をワークスペースの記録より先に返す。アドレスの Store 名の Store がなく、main に
// 接頭辞 QQ のワークスペースの記録(team の Team を指す。Team の接頭辞の履歴に QQ はないので (a)(b) では見つからない)が
// あると、QQ-1 は記録で team の TM-1 を開く。readonly に QQ-1 の種類の記録(完全に削除したもの)もあれば、TM-1 ではなく
// purged を返す。実際の MoveWorkspace の記録では、移したワークスペースが記録の old をいまの接頭辞か旧い接頭辞に持つので、
// ほかの Store では (a)(b) が先に当たる。そのため記録を直接書いて順を確かめる。
func TestLocateItemRecordBeforeWorkspaceRecordInOtherStores(t *testing.T) {
	f := newFixture(t)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityWorkspace, Target: teamWS, Old: "QQ", New: "QQ", ToStore: string(store2), ToStoreName: "team"})
	})
	if got, err := f.svc.Locate(t.Context(), "nope", "QQ-1"); err != nil || !got.Found || got.Store != store2 || got.Item != teamEpic {
		t.Fatalf("ワークスペースの記録だけ: %+v, %v", got, err)
	}
	const gone item.ID = "01TESTIT000000000000000052"
	putRecord(t, f, store3, gone, "QQ-1", "AR-9", store2, "team")
	f.rec.AddPurged(store2, port.Purged{Entity: transfer.EntityItem, Target: gone, Display: "AR-9", At: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)})
	got, err := f.svc.Locate(t.Context(), "nope", "QQ-1")
	if err != nil || got.Found || got.Gone == nil || got.Gone.Reason != "purged" || got.Gone.Old != "QQ-1" || got.Gone.NewDisplay != "AR-9" {
		t.Errorf("種類の記録もある: %+v, %+v, %v", got, got.Gone, err)
	}
}
