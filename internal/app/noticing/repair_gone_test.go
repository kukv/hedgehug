package noticing_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/relation"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	item9  item.ID = "01PLACEIT00000000000000009"
	item8  item.ID = "01PLACEIT00000000000000008"
	item31 item.ID = "01PLACEIT00000000000000031"
	goneWS         = workspace.ID("01GONEWS000000000000000001")
)

var purgedAt = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

// logOfRef は、ref のいまのログ。
func (f *fixture) logOfRef(ref change.Ref) []change.Record {
	f.t.Helper()
	var log []change.Record
	if err := f.stores.Read(f.t.Context(), store, func(r port.Reader) (err error) {
		log, err = r.Changes().Of(ref)
		return err
	}); err != nil {
		f.t.Fatal(err)
	}
	return log
}

// purge は、purged.jsonl に target(画面の ID は display)の行を足す。
func (f *fixture) purge(kind string, target change.ID, display string) {
	f.stores.AddPurged(store, port.Purged{Entity: kind, Target: target, Display: display, At: purgedAt})
}

// purgeUnder は、ref を完全に削除した修復 (1) の保存(行がなく、消える前のログを持つ)。
func purgeUnder(f *fixture, ref change.Ref) port.ReceivedSave {
	f.t.Helper()
	return port.ReceivedSave{
		Key: port.SaveKey{Version: "purge", At: purgedAt}, Repair: true,
		Removed: []port.RemovedEntity{{Ref: ref, Log: f.logOfRef(ref)}},
	}
}

// dropped は、合わせで ref が消えた合わせ。New は、newRows の保存の行(その側の、共通の祖先にない行)。
func dropped(f *fixture, ref change.Ref, newRows ...port.ReceivedSave) port.ReceivedSave {
	f.t.Helper()
	var rows []change.Record
	for _, s := range newRows {
		for _, e := range s.Entities {
			if e.Ref == ref {
				rows = append(rows, e.Rows...)
			}
		}
	}
	return port.ReceivedSave{
		Key: port.SaveKey{Version: "merge"}, Merge: true,
		Removed: []port.RemovedEntity{{Ref: ref, Log: f.logOfRef(ref), New: rows}},
	}
}

// retitle は、by が id のタイトルを「直した」にする保存。
func retitle(f *fixture, by string, id item.ID) port.ReceivedSave {
	f.t.Helper()
	return f.save(user(by), appendRef(itemRefOf(id), change.NewSet("title", "子", "直した")))
}

func TestPurgedUnderNotice(t *testing.T) {
	f := setup(t)
	addItem(f, "taro", webItem(item7, 7, item.Task, apptest.SampleEpic))
	retitle(f, "hanako", item7)
	f.purge(transfer.EntityItem, apptest.SampleEpic, "HH-1")
	f.judge(purgeUnder(f, itemRefOf(item7)))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Item: item7, Display: "HH-7",
		Text:   "HH-1 は完全に削除されたため、あなたの変更は反映されませんでした",
		Source: "repair:purged:" + string(item7),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// 祖先がプロジェクト: 近い方から探し、プロジェクトの記録の ID を使う。
func TestPurgedUnderProject(t *testing.T) {
	f := setup(t)
	addItem(f, "hanako", webItem(item7, 7, item.Task, apptest.SampleEpic))
	f.purge(transfer.EntityProject, apptest.SampleProject, "HH-P1")
	f.judge(purgeUnder(f, itemRefOf(item7)))
	if n := f.wantOne(); n.Text != "HH-P1 は完全に削除されたため、あなたの変更は反映されませんでした" {
		t.Errorf("text = %q", n.Text)
	}
}

func TestPurgedUnderOtherSaver(t *testing.T) {
	f := setup(t)
	addItem(f, "hanako", webItem(item7, 7, item.Task, apptest.SampleEpic))
	retitle(f, "taro", item7)
	f.purge(transfer.EntityItem, apptest.SampleEpic, "HH-1")
	f.judge(purgeUnder(f, itemRefOf(item7)))
	f.wantNone()
}

// relating は、taro が HH-9 を作り、adder が HH-2 から HH-9 への関連を足し、HH-9 が完全に削除されたあとの場面。
func relating(f *fixture, adder string) item.Relation {
	f.t.Helper()
	addItem(f, "taro", webItem(item9, 9, item.Task, apptest.SampleEpic))
	rel := item.Relation{Type: relation.TypeBlocks, To: item9}
	f.save(user(adder), appendRef(itemRefOf(apptest.SampleTask), change.NewAdd("relations", rel)))
	f.purge(transfer.EntityItem, item9, "HH-9")
	return rel
}

func TestPurgedRelationNotice(t *testing.T) {
	f := setup(t)
	rel := relating(f, "hanako")
	f.judge(f.repairSave(appendRef(itemRefOf(apptest.SampleTask), change.NewRemove("relations", rel))))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask, Display: "HH-2",
		Text:   "HH-9 は完全に削除されたため、あなたの変更は反映されませんでした",
		Source: "repair:relation:" + string(apptest.SampleTask) + ":" + string(item9),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestPurgedRelationOtherAdder(t *testing.T) {
	f := setup(t)
	rel := relating(f, "taro")
	f.judge(f.repairSave(appendRef(itemRefOf(apptest.SampleTask), change.NewRemove("relations", rel))))
	f.wantNone()
}

func TestPurgedDropped(t *testing.T) {
	f := setup(t)
	addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
	edit := retitle(f, "hanako", item5)
	f.purge(transfer.EntityItem, item5, "HH-5")
	f.judge(dropped(f, itemRefOf(item5), edit))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Item: item5, Display: "HH-5",
		Text:   "HH-5 は完全に削除されたため、あなたの変更は反映されませんでした",
		Source: "repair:purged:" + string(item5),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestDroppedOnlyNewRows(t *testing.T) {
	f := setup(t)
	addItem(f, "hanako", webItem(item5, 5, item.Task, apptest.SampleEpic))
	edit := retitle(f, "jiro", item5)
	f.purge(transfer.EntityItem, item5, "HH-5")
	f.judge(dropped(f, itemRefOf(item5), edit))
	f.wantNone()
}

// moving は、taro(表示名 太郎)が HH-5 を Store「team-b」の WEB-12 へ移した転送記録を書く。
func moving(f *fixture, toStore string) {
	f.t.Helper()
	movingRecord(f, transfer.Record{
		Entity: transfer.EntityItem, Target: item5, Old: "HH-5", New: "WEB-12", ToStore: toStore, ToStoreName: "team-b",
	})
}

// movingRecord は、taro(表示名 太郎)の転送記録 rec を書く。
func movingRecord(f *fixture, rec transfer.Record) {
	f.t.Helper()
	f.save(user("taro"), func(tx port.Tx) error {
		if err := tx.WriteMembers().Append(member.Rename("taro", "taro", "太郎")); err != nil {
			return err
		}
		return tx.WriteTransfers().Append(rec)
	})
}

func TestMovedDropped(t *testing.T) {
	f := setup(t)
	addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
	edit := retitle(f, "hanako", item5)
	moving(f, "S2")
	f.judge(dropped(f, itemRefOf(item5), edit))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Item: item5, Display: "HH-5",
		Text:   "HH-5 は 太郎 が team-b へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-12 です",
		Source: "repair:moved:" + string(item5),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// 同じ Store の中の転送記録は、別の Store への移動に数えない。
func TestMovedSameStoreNotCounted(t *testing.T) {
	f := setup(t)
	addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
	edit := retitle(f, "hanako", item5)
	moving(f, string(store))
	f.judge(dropped(f, itemRefOf(item5), edit))
	f.wantNone()
}

func TestMovedChildDropped(t *testing.T) {
	f := setup(t)
	addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
	moving(f, "S2")
	child := webItem(item31, 31, item.Step, item5)
	child.Title = "リリースノート"
	create := f.save(user("hanako"), appendRef(itemRefOf(item31), item.Create(child)))
	f.judge(dropped(f, itemRefOf(item31), create))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Item: item31, Display: "HH-31",
		Text:   "HH-5 は 太郎 が team-b へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-12 です(HH-31「リリースノート」)",
		Source: "repair:moved:" + string(item31),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestDroppedNoCause(t *testing.T) {
	f := setup(t)
	addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
	edit := retitle(f, "hanako", item5)
	f.judge(dropped(f, itemRefOf(item5), edit))
	f.wantNone()
}

// プロジェクトは Project と Display、ワークスペースは Workspace だけを持つ。ワークスペースが原因なら、{ID} は
// 消える前のログの名前の「ワークスペース「{名前}」」。
func TestDroppedProjectAndWorkspace(t *testing.T) {
	f := setup(t)
	pref := change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}
	wref := change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}
	pedit := f.save(user("HANAKO"), appendRef(pref, change.NewSet("title", "", "直した")))
	wedit := f.save(user("hanako"), appendRef(wref, change.NewSet("name", "Web", "ウェブ")))
	f.purge(transfer.EntityWorkspace, apptest.SampleWorkspace, "Web")
	merge := dropped(f, pref, pedit)
	merge.Removed = append(merge.Removed, dropped(f, wref, wedit).Removed...)
	f.judge(merge)
	got := f.notices()
	want := []port.Notice{
		{
			Kind: port.NoticeRepair, Store: store, Project: apptest.SampleProject, Display: project.Project{Place: project.Place{Number: 1}}.DisplayID("HH"),
			Text: "ワークスペース「ウェブ」は完全に削除されたため、あなたの変更は反映されませんでした", Source: "repair:purged:" + string(apptest.SampleProject),
		},
		{
			Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace,
			Text: "ワークスペース「ウェブ」は完全に削除されたため、あなたの変更は反映されませんでした", Source: "repair:purged:" + string(apptest.SampleWorkspace),
		},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("notices = %+v, want %+v", got, want)
	}
}

func TestDroppedReadOnly(t *testing.T) {
	f := setup(t)
	addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
	edit := retitle(f, "hanako", item5)
	f.purge(transfer.EntityItem, item5, "HH-5")
	if err := f.judgeReceived(port.Received{Saves: []port.ReceivedSave{dropped(f, itemRefOf(item5), edit)}, ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	f.wantNone()
}

// 修復 (1) は 1 件ずつ消す(HH-1 > HH-7 > HH-8): HH-8 の本文は、先に消えた HH-7 を書く。
func TestPurgedUnderNested(t *testing.T) {
	f := setup(t)
	addItem(f, "hanako", webItem(item7, 7, item.Task, apptest.SampleEpic))
	addItem(f, "hanako", webItem(item8, 8, item.Step, item7))
	f.purge(transfer.EntityItem, apptest.SampleEpic, "HH-1")
	first := purgeUnder(f, itemRefOf(item7))
	f.purge(transfer.EntityItem, item7, "HH-7")
	f.judge(first, purgeUnder(f, itemRefOf(item8)))
	got := f.notices()
	if len(got) != 2 || got[0].Item != item7 || got[1].Item != item8 ||
		got[0].Text != "HH-1 は完全に削除されたため、あなたの変更は反映されませんでした" ||
		got[1].Text != "HH-7 は完全に削除されたため、あなたの変更は反映されませんでした" {
		t.Errorf("notices = %+v", got)
	}
}

// 完全に削除されたワークスペースのログがなければ、名前は記録の display。
func TestPurgedUnderWorkspaceNoLog(t *testing.T) {
	f := setup(t)
	it := webItem(item7, 7, item.Task, "")
	it.Place.Workspace, it.Place.Project = goneWS, ""
	f.save(user("hanako"), appendRef(itemRefOf(item7), item.Create(it)))
	f.purge(transfer.EntityWorkspace, goneWS, "Old")
	f.judge(purgeUnder(f, itemRefOf(item7)))
	if n := f.wantOne(); n.Text != "ワークスペース「Old」は完全に削除されたため、あなたの変更は反映されませんでした" || n.Display != string(item7) {
		t.Errorf("notice = %+v", n)
	}
}

func TestMovedProjectDropped(t *testing.T) {
	f := setup(t)
	pref := change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}
	edit := f.save(user("hanako"), appendRef(pref, change.NewSet("title", "", "直した")))
	movingRecord(f, transfer.Record{
		Entity: transfer.EntityProject, Target: apptest.SampleProject, Old: "HH-P1", New: "WEB-P3", ToStore: "S2", ToStoreName: "team-b",
	})
	f.judge(dropped(f, pref, edit))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Project: apptest.SampleProject, Display: "HH-P1",
		Text:   "HH-P1 は 太郎 が team-b へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-P3 です",
		Source: "repair:moved:" + string(apptest.SampleProject),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// ワークスペースが別の Store へ移った: 「ワークスペース「{名前}」」と書き、「。移動先は…です」を付けない。
func TestMovedWorkspaceChildDropped(t *testing.T) {
	f := setup(t)
	addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
	edit := retitle(f, "hanako", item5)
	movingRecord(f, transfer.Record{
		Entity: transfer.EntityWorkspace, Target: apptest.SampleWorkspace, Old: "HH", New: "WEB", ToStore: "S2", ToStoreName: "team-b",
	})
	f.judge(dropped(f, itemRefOf(item5), edit))
	if n := f.wantOne(); n.Text != "ワークスペース「Web」は 太郎 が team-b へ移動したため、あなたの変更は反映されませんでした(HH-5「直した」)" {
		t.Errorf("text = %q", n.Text)
	}
}

// 移動先の Store が表示中なら、このマシンでの名前を使う。なければ to_store_name、それも空なら to_store。
func TestMovedStoreName(t *testing.T) {
	for name, c := range map[string]struct {
		rec  transfer.Record
		want string
	}{
		"表示中":    {transfer.Record{ToStore: "S2", ToStoreName: "team-b"}, "team-local"},
		"名前が空":   {transfer.Record{ToStore: "S3"}, "S3"},
		"表示中でない": {transfer.Record{ToStore: "S3", ToStoreName: "team-c"}, "team-c"},
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			f.svc.List = apptest.StoreList{Stores: []port.StoreInfo{{ID: store, Name: "main"}, {ID: "S2", Name: "team-local"}}}
			addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
			edit := retitle(f, "hanako", item5)
			rec := c.rec
			rec.Entity, rec.Target, rec.Old, rec.New = transfer.EntityItem, item5, "HH-5", "WEB-12"
			movingRecord(f, rec)
			f.judge(dropped(f, itemRefOf(item5), edit))
			if n := f.wantOne(); n.Text != "HH-5 は 太郎 が "+c.want+" へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-12 です" {
				t.Errorf("text = %q", n.Text)
			}
		})
	}
}

// counting は、purged.jsonl と transfers.jsonl を読んだ回数を数える Reader。
type counting struct {
	port.Reader
	purged, transfers *int
}

func (c counting) Purged() port.PurgedReader {
	*c.purged++
	return c.Reader.Purged()
}

func (c counting) Transfers() port.TransferReader {
	*c.transfers++
	return c.Reader.Transfers()
}

// 記録は、1 回の判定で 1 回だけ読む。
func TestCausesReadOnce(t *testing.T) {
	f := setup(t)
	rel := relating(f, "hanako")
	addItem(f, "taro", webItem(item5, 5, item.Task, apptest.SampleEpic))
	addItem(f, "taro", webItem(item7, 7, item.Task, apptest.SampleEpic))
	e5, e7 := retitle(f, "hanako", item5), retitle(f, "hanako", item7)
	f.purge(transfer.EntityItem, item5, "HH-5")
	f.purge(transfer.EntityItem, item7, "HH-7")
	merge := dropped(f, itemRefOf(item5), e5)
	merge.Removed = append(merge.Removed, dropped(f, itemRefOf(item7), e7).Removed...)
	unlink := f.repairSave(appendRef(itemRefOf(apptest.SampleTask), change.NewRemove("relations", rel)))
	var np, nt int
	ctx := t.Context()
	err := f.svc.Judge(ctx, store, port.Received{Saves: []port.ReceivedSave{unlink, merge}}, func(fn func(r port.Reader) error) error {
		return f.stores.Read(ctx, store, func(r port.Reader) error { return fn(counting{r, &np, &nt}) })
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.notices()) != 3 || np != 1 || nt != 1 {
		t.Errorf("notices = %d, purged reads = %d, transfer reads = %d, want 3, 1, 1", len(f.notices()), np, nt)
	}
}
