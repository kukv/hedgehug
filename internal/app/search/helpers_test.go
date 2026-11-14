package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/query"
)

const (
	store1 port.StoreID = "S1" // main
	store2 port.StoreID = "S2" // team(閲覧のみ)
	store3 port.StoreID = "S3" // 非表示(StoreList に出ない)

	item3 item.ID = "01TESTIT000000000000000003"
	item4 item.ID = "01TESTIT000000000000000004"
	item5 item.ID = "01TESTIT000000000000000005"
	item6 item.ID = "01TESTIT000000000000000006"
)

// clockStart は、Service の時計の最初の時刻。
var clockStart = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

type fixture struct {
	stores *apptest.Stores
	inbox  *apptest.Inbox
	svc    Service
}

// newFixture は、S1・S2・S3 を作り、表示中の Store を S1「main」・S2「team」(閲覧のみ)にする。S3 は非表示。
// 「あなた」はどの Store でも taro。中身は各テストが書く。
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
	inbox := &apptest.Inbox{}
	return fixture{stores: stores, inbox: inbox, svc: Service{
		Stores: stores, StoreList: list, SyncTargets: targets, Identity: apptest.Identity{Login: "taro"},
		Inbox: inbox, Clock: apptest.NewClock(clockStart),
	}}
}

// write は、Store S1 に 1 回の保存で書く。
func write(t *testing.T, s port.Stores, fn func(tx port.Tx) error) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	if err := s.Write(t.Context(), store1, op, fn); err != nil {
		t.Fatal(err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// addItem は、Store S1 にアイテムを作る(作成の行と、番号の行)。
func addItem(t *testing.T, s port.Stores, it item.Item) {
	t.Helper()
	write(t, s, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(it.ID, item.Create(it)),
			tx.WriteWorkspaces().Append(it.Place.Workspace, numbering.Draft(numbering.Items, it.Place.Number)),
		)
	})
}

// task は、サンプルの「Web」のプロジェクトの、Todo のアイテム。parent が nil でなければその下。
func task(id item.ID, n int, kind item.Kind, parent *item.ID, title string) item.Item {
	return item.Item{
		ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: parent, Number: n},
		Kind: kind, Rank: "V", Title: title, Status: apptest.SampleStatusTodo,
	}
}

func ptr(id item.ID) *item.ID { return &id }

// parse は、クエリを解析する(追加プロパティのキーはない)。
func parse(t *testing.T, text string) query.Query {
	t.Helper()
	q, errs := query.Parse(text, func(string) bool { return false })
	if len(errs) > 0 {
		t.Fatalf("Parse(%q) = %v", text, errs)
	}
	return q
}

func entriesOf(t *testing.T, f fixture, text string) []entry {
	t.Helper()
	got, err := f.svc.entries(t.Context(), parse(t, text))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// factsOf は、Store store の表示 ID display のアイテムの材料。なければテストを止める。
func factsOf(t *testing.T, es []entry, store port.StoreID, display string) query.Facts {
	t.Helper()
	for _, e := range es {
		if e.info.ID == store && e.facts.Display == display {
			return e.facts
		}
	}
	t.Fatalf("%s/%s がない", store, display)
	return query.Facts{}
}

// displays は、entries の「Store/表示 ID」を並びのまま返す。
func displays(es []entry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, string(e.info.ID)+"/"+e.facts.Display)
	}
	return out
}

// logOf は、アイテムのログ。
func logOf(t *testing.T, s port.Stores, store port.StoreID, id item.ID) []change.Record {
	t.Helper()
	var log []change.Record
	must(t, s.Read(t.Context(), store, func(r port.Reader) (err error) {
		log, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		return err
	}))
	return log
}

// failingInbox は、呼ばれたらテストを失敗にする受信箱。
type failingInbox struct {
	port.InboxBook
	t *testing.T
}

func (f failingInbox) List(context.Context, port.NoticeQuery) ([]port.NoticeRow, error) {
	f.t.Error("受信箱を読んだ")
	return nil, nil
}
