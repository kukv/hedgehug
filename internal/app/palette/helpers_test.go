package palette_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/palette"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	store1 port.StoreID = "S1" // main
	store2 port.StoreID = "S2" // team(閲覧のみ)
	store3 port.StoreID = "S3" // StoreList に出ない

	item12   item.ID      = "01TESTIT000000000000000012"
	gone     item.ID      = "01TESTIT000000000000000013"
	under    item.ID      = "01TESTIT000000000000000014"
	moved    item.ID      = "01TESTIT000000000000000015"
	webTask  item.ID      = "01TESTIT000000000000000016"
	project2 project.ID   = "01TESTPR000000000000000002"
	webProj  project.ID   = "01TESTPR000000000000000003"
	webWS    workspace.ID = "01TESTWS000000000000000003"
)

// clockStart は、Service の時計の最初の時刻(Now を呼ぶたびに 1ms 進む)。
var clockStart = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

type fixture struct {
	stores *apptest.Stores
	recent *fakeRecent
	svc    palette.Service
}

// newFixture は、S1・S2・S3 を作り、表示中の Store を S1「main」・S2「team」(閲覧のみ)にする。このマシンの Store
// (SyncTargets)は S1・S2 と、非表示の S3。中身は各テストが書く。
func newFixture(t *testing.T) fixture {
	t.Helper()
	stores := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	for _, id := range []port.StoreID{store1, store2, store3} {
		stores.AddStore(id)
	}
	list := &apptest.StoreList{Stores: []port.StoreInfo{{ID: store1, Name: "main"}, {ID: store2, Name: "team", ReadOnly: true}}}
	targets := &apptest.SyncTargets{Targets: []port.SyncTarget{
		{ID: store1, Name: "main"}, {ID: store2, Name: "team", ReadOnly: true}, {ID: store3, Name: "old", Hidden: true},
	}}
	recent := &fakeRecent{}
	return fixture{stores: stores, recent: recent, svc: palette.Service{
		Stores: stores, StoreList: list, SyncTargets: targets, RecentItems: recent, Clock: apptest.NewClock(clockStart),
	}}
}

func write(t *testing.T, s port.Stores, store port.StoreID, fn func(tx port.Tx) error) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	if err := s.Write(t.Context(), store, op, fn); err != nil {
		t.Fatal(err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// addItem は、Store store にアイテムを作る(作成の行と、番号の行)。
func addItem(t *testing.T, s port.Stores, store port.StoreID, it item.Item) {
	t.Helper()
	write(t, s, store, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(it.ID, item.Create(it)),
			tx.WriteWorkspaces().Append(it.Place.Workspace, numbering.Draft(numbering.Items, it.Place.Number)),
		)
	})
}

func addProject(t *testing.T, s port.Stores, store port.StoreID, p project.Project) {
	t.Helper()
	write(t, s, store, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(p.ID, project.Create(p)),
			tx.WriteWorkspaces().Append(p.Place.Workspace, numbering.Draft(numbering.Projects, p.Place.Number)),
		)
	})
}

func task(id item.ID, ws workspace.ID, p project.ID, n int, title string) item.Item {
	return item.Item{ID: id, Place: item.Place{Workspace: ws, Project: p, Number: n}, Kind: item.Task, Rank: "V", Title: title}
}
