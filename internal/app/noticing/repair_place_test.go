package noticing_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	item3 item.ID    = "01PLACEIT00000000000000003"
	item5 item.ID    = "01PLACEIT00000000000000005"
	item6 item.ID    = "01PLACEIT00000000000000006"
	proj2 project.ID = "01PLACEPR00000000000000002"
)

func itemRefOf(id item.ID) change.Ref { return change.Ref{Kind: change.KindItem, ID: id} }

// addItem は、by が Web(HH)に it を作る保存。番号も使ったことにする。
func addItem(f *fixture, by string, it item.Item) {
	f.t.Helper()
	f.save(user(by), func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(it.Place.Workspace, numbering.Draft(numbering.Items, it.Place.Number)),
			tx.WriteItems().Append(it.ID, item.Create(it)),
		)
	})
}

// webItem は、Web の HH-1 のプロジェクトの、番号 n のアイテム。parent が空ならプロジェクトの直下。
func webItem(id item.ID, n int, kind item.Kind, parent item.ID) item.Item {
	p := item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: n}
	if parent != "" {
		p.Parent = &parent
	}
	return item.Item{ID: id, Place: p, Kind: kind, Rank: rank.First, Title: "子"}
}

// setPlace は、id の場所を to にする行(いまの場所から)を、actor で保存する。
func setPlace(f *fixture, actor change.Actor, id item.ID, to func(p item.Place) item.Place) port.ReceivedSave {
	f.t.Helper()
	save := f.save
	if actor.Kind == change.ActorSync {
		save = func(_ change.Actor, fn func(tx port.Tx) error) port.ReceivedSave { return f.repairSave(fn) }
	}
	return save(actor, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		return tx.WriteItems().Append(id, change.NewSet("place", it.Place, to(it.Place)))
	})
}

func number(n int) func(item.Place) item.Place {
	return func(p item.Place) item.Place { p.Number = n; return p }
}

func parentTo(id item.ID) func(item.Place) item.Place {
	return func(p item.Place) item.Place { p.Parent = &id; return p }
}

var syncActor = change.Actor{Kind: change.ActorSync}

func TestNumberNotice(t *testing.T) {
	f := setupAs(t, "taro")
	addItem(f, "taro", webItem(item3, 3, item.Epic, ""))
	f.judge(f.repairSave(func(tx port.Tx) error {
		it, err := tx.Items().Get(item3)
		if err != nil {
			return err
		}
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, 4)),
			tx.WriteItems().Append(item3, change.NewSet("place", it.Place, number(4)(it.Place))),
		)
	}))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Item: item3,
		Text: "2 台で同時に HH-3 が作られたため、あとから届いた方を HH-4 に振り直しました", Display: "HH-4",
		Source: "repair:number:" + string(item3) + ":4",
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestNumberNoticeProject(t *testing.T) {
	f := setupAs(t, "taro")
	p := project.Project{ID: proj2, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "サイト", Status: project.Todo}
	f.save(user("taro"), func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Projects, 2)),
			tx.WriteProjects().Append(proj2, project.Create(p)),
		)
	})
	to := p.Place
	to.Number = 3
	f.judge(f.repairSave(appendRef(change.Ref{Kind: change.KindProject, ID: proj2}, change.NewSet("place", p.Place, to))))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Project: proj2,
		Text: "2 台で同時に HH-P2 が作られたため、あとから届いた方を HH-P3 に振り直しました", Display: "HH-P3",
		Source: "repair:number:" + string(proj2) + ":3",
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestNumberNoticeOtherCreator(t *testing.T) {
	f := setupAs(t, "taro")
	addItem(f, "hanako", webItem(item3, 3, item.Epic, ""))
	f.judge(setPlace(f, syncActor, item3, number(4)))
	f.wantNone()
}

// cycling は、hanako が HH-5(Step、HH-2 の下)を作り、placer が親を HH-2 に付け直したあとの場面。
func cycling(f *fixture, placer string) {
	f.t.Helper()
	addItem(f, "hanako", webItem(item5, 5, item.Step, apptest.SampleTask))
	setPlace(f, user(placer), item5, parentTo(apptest.SampleTask))
}

func TestCycleNotice(t *testing.T) {
	f := setupAs(t, "taro")
	cycling(f, "taro")
	f.judge(setPlace(f, syncActor, item5, parentTo(apptest.SampleEpic)))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Item: item5,
		Text: "親が循環したため、HH-5 を HH-1 の下へ移しました", Display: "HH-5",
		Source: "repair:cycle:" + string(item5) + ":" + string(apptest.SampleEpic),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestCycleNoticeStepToTask(t *testing.T) {
	f := setupAs(t, "taro")
	cycling(f, "taro")
	f.judge(f.repairSave(func(tx port.Tx) error {
		it, err := tx.Items().Get(item5)
		if err != nil {
			return err
		}
		return tx.WriteItems().Append(item5,
			change.NewSet("place", it.Place, parentTo(apptest.SampleEpic)(it.Place)),
			change.NewSet("rank", it.Rank, rank.After(it.Rank)),
			change.NewSet("kind", item.Step, item.Task))
	}))
	if n := f.wantOne(); n.Text != "親が循環したため、HH-5 を HH-1 の下へ移しました。Step は Task になりました" {
		t.Errorf("text = %q", n.Text)
	}
}

// 行き先がプロジェクトの直下: {行き先の ID} はプロジェクトの ID。
func TestCycleNoticeToProject(t *testing.T) {
	f := setupAs(t, "taro")
	cycling(f, "taro")
	f.judge(setPlace(f, syncActor, item5, func(p item.Place) item.Place { p.Parent = nil; return p }))
	n := f.wantOne()
	if n.Text != "親が循環したため、HH-5 を HH-P1 の下へ移しました" || n.Source != "repair:cycle:"+string(item5)+":"+string(apptest.SampleProject) {
		t.Errorf("notice = %+v", n)
	}
}

func TestCycleNoticeOtherPlacer(t *testing.T) {
	f := setupAs(t, "taro")
	cycling(f, "hanako")
	f.judge(setPlace(f, syncActor, item5, parentTo(apptest.SampleEpic)))
	f.wantNone()
}

// 子を作った人(作成の行は場所も書く)は、場所を保存した人。
func TestCycleNoticeCreator(t *testing.T) {
	f := setupAs(t, "taro")
	addItem(f, "taro", webItem(item5, 5, item.Step, apptest.SampleTask))
	f.judge(setPlace(f, syncActor, item5, parentTo(apptest.SampleEpic)))
	if n := f.wantOne(); n.Text != "親が循環したため、HH-5 を HH-1 の下へ移しました" {
		t.Errorf("text = %q", n.Text)
	}
}

// siteProject は、Web にプロジェクト HH-P2「サイト」を作り、hanako が HH-2 をそこへ移し、hanako が作った
// HH-6(Step)の親を placer が HH-2 に付けたあとの場面。
func siteProject(f *fixture, placer string) {
	f.t.Helper()
	p := project.Project{ID: proj2, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "サイト", Status: project.Todo}
	f.save(user("hanako"), func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Projects, 2)),
			tx.WriteProjects().Append(proj2, project.Create(p)),
		)
	})
	addItem(f, "hanako", webItem(item6, 6, item.Step, apptest.SampleEpic))
	setPlace(f, user("hanako"), apptest.SampleTask, func(p item.Place) item.Place { p.Project, p.Parent = proj2, nil; return p })
	setPlace(f, user(placer), item6, parentTo(apptest.SampleTask))
}

func toSite(p item.Place) item.Place { p.Project = proj2; return p }

func TestPlaceProjectNotice(t *testing.T) {
	f := setupAs(t, "taro")
	siteProject(f, "taro")
	f.judge(setPlace(f, syncActor, item6, toSite))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Item: item6,
		Text: "HH-2 が プロジェクト「サイト」へ移動したため、HH-6 をそのプロジェクトに移しました", Display: "HH-6",
		Source: "repair:place:" + string(item6) + ":" + string(apptest.SampleWorkspace) + ":" + string(proj2),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestPlaceProjectNoticeOtherPlacer(t *testing.T) {
	f := setupAs(t, "taro")
	siteProject(f, "hanako")
	f.judge(setPlace(f, syncActor, item6, toSite))
	f.wantNone()
}

const (
	docsWS   workspace.ID = "01DOCSWS000000000000000001"
	docsProj project.ID   = "01DOCSPR000000000000000001"
	item7    item.ID      = "01PLACEIT00000000000000007"
)

// docsValue は、Docs の値の内部 ID(DefaultSettings が振る順の n 番目。8 は優先度 Medium、11 は関連システム Web)。
func docsValue(n int) change.ID { return change.ID(fmt.Sprintf("01DOCSVAL%017d", n)) }

// straying は、ワークスペース「Docs」(DOC、プロジェクト DOC-P1、優先度 Medium の名前は「中」、関連システム Web の
// 名前は「ウェブ」)を作り、taro が HH-2 の下に HH-6(優先度 Medium・関連システム Web)と、その下の削除済みの
// HH-7 を作り、hanako が HH-2 を DOC-3 へ移したあとの場面。record なら、その移動の転送記録(HH-2 → DOC-3)も書く。
func straying(f *fixture, record bool) {
	f.t.Helper()
	f.save(user("hanako"), func(tx port.Tx) error {
		n := 0
		st := workspace.DefaultSettings(func() change.ID { n++; return docsValue(n) })
		d, err := workspace.CreateWithSettings(workspace.Workspace{ID: docsWS, Name: "Docs", Prefix: "DOC"}, st)
		if err != nil {
			return err
		}
		p := project.Project{ID: docsProj, Place: project.Place{Workspace: docsWS, Number: 1}, Rank: "V", Title: "資料", Status: project.Todo}
		return errors.Join(
			tx.WriteWorkspaces().Append(docsWS, d,
				change.NewSet("priorities/"+string(docsValue(8))+"/name", "Medium", "中"),
				change.NewSet("systems/"+string(docsValue(11))+"/name", "Web", "ウェブ"),
				numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1), numbering.Draft(numbering.Items, 2)),
			tx.WriteProjects().Append(docsProj, project.Create(p)),
		)
	})
	child := webItem(item6, 6, item.Step, apptest.SampleTask)
	child.Priority, child.Systems = apptest.SamplePriorityMedium, []change.ID{apptest.SampleSystemWeb}
	addItem(f, "taro", child)
	addItem(f, "taro", webItem(item7, 7, item.Step, item6))
	f.save(user("taro"), appendRef(itemRefOf(item7), change.NewDeleteEntity(true)))
	f.save(user("hanako"), func(tx port.Tx) error {
		it, err := tx.Items().Get(apptest.SampleTask)
		if err != nil {
			return err
		}
		errs := []error{
			tx.WriteWorkspaces().Append(docsWS, numbering.Draft(numbering.Items, 3)),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("place", it.Place, item.Place{Workspace: docsWS, Project: docsProj, Number: 3})),
		}
		if record {
			errs = append(errs, tx.WriteTransfers().Append(transfer.Record{
				Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "HH-2", New: "DOC-3", ToStore: string(store),
			}))
		}
		return errors.Join(errs...)
	})
}

// strayRepair は、HH-6 を DOC-4、HH-7 を DOC-5 へ移す修復の保存(値の引き直しの行は本文に効かないので書かない)。
func strayRepair(f *fixture) port.ReceivedSave {
	f.t.Helper()
	return f.repairSave(func(tx port.Tx) error {
		errs := []error{tx.WriteWorkspaces().Append(docsWS, numbering.Draft(numbering.Items, 4), numbering.Draft(numbering.Items, 5))}
		for _, m := range []struct {
			id item.ID
			n  int
		}{{item6, 4}, {item7, 5}} {
			it, err := tx.Items().Get(m.id)
			if err != nil {
				return err
			}
			to := item.Place{Workspace: docsWS, Project: docsProj, Parent: it.Place.Parent, Number: m.n}
			errs = append(errs, tx.WriteItems().Append(m.id, change.NewSet("place", it.Place, to)))
		}
		return errors.Join(errs...)
	})
}

func TestPlaceWorkspaceNotice(t *testing.T) {
	f := setupAs(t, "taro")
	straying(f, true)
	f.judge(strayRepair(f))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: docsWS, Item: item6,
		Text:    "HH-2 が Docs へ移動したため、HH-6 を DOC-4 に移動しました。合う値がなく空になった項目: 優先度・関連システム",
		Display: "DOC-4", Source: "repair:place:" + string(item6) + ":" + string(docsWS) + ":" + string(docsProj),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// 親の転送記録がない: {親の ID} は親のいまの表示。
func TestPlaceWorkspaceNoticeNoTransferRecord(t *testing.T) {
	f := setupAs(t, "taro")
	straying(f, false)
	f.judge(strayRepair(f))
	if n := f.wantOne(); n.Text != "DOC-3 が Docs へ移動したため、HH-6 を DOC-4 に移動しました。合う値がなく空になった項目: 優先度・関連システム" {
		t.Errorf("text = %q", n.Text)
	}
}

// 修復のあと、同じ判定の中で親がまた移った: そのあとの転送記録は使わない。
func TestPlaceWorkspaceNoticeLaterRecord(t *testing.T) {
	f := setupAs(t, "taro")
	straying(f, true)
	s := strayRepair(f)
	later := f.save(user("hanako"), func(tx port.Tx) error {
		return tx.WriteTransfers().Append(transfer.Record{
			Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "DOC-3", New: "HH-9", ToStore: string(store),
		})
	})
	f.judge(s, later)
	if n := f.wantOne(); !strings.HasPrefix(n.Text, "HH-2 が Docs へ") {
		t.Errorf("text = %q", n.Text)
	}
}

// 別の Store への転送記録は、親の移動の記録に使わない。
func TestPlaceWorkspaceNoticeOtherStoreRecord(t *testing.T) {
	f := setupAs(t, "taro")
	straying(f, false)
	f.save(user("hanako"), func(tx port.Tx) error {
		return tx.WriteTransfers().Append(transfer.Record{
			Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "XX-9", New: "TM-9", ToStore: "S2",
		})
	})
	f.judge(strayRepair(f))
	if n := f.wantOne(); !strings.HasPrefix(n.Text, "DOC-3 が Docs へ") {
		t.Errorf("text = %q", n.Text)
	}
}

// 2 台が同じ移動をした: 2 つ目の修復の保存は、同じ移動(Web の HH-6 → DOC-4)の set place の行を、もう一度書く。
func TestPlaceOnceAcrossMachines(t *testing.T) {
	f := setupAs(t, "taro")
	straying(f, true)
	s := strayRepair(f)
	other := f.repairSave(func(tx port.Tx) error {
		it, err := tx.Items().Get(item6)
		if err != nil {
			return err
		}
		from := it.Place
		from.Workspace, from.Number = apptest.SampleWorkspace, 6
		return tx.WriteItems().Append(item6, change.NewSet("place", from, it.Place))
	})
	f.judge(s, other)
	f.judge(s, other)
	if rows := f.rows(); len(rows) != 1 {
		t.Errorf("rows = %+v, want one", rows)
	}
}
