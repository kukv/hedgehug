package transfers_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// S1「main」はサンプル(Web・HH)と Mobile(MB)、S2「team」は Team(TM)、S3「readonly」は閲覧のみで Archive(AR)。
// S1 は remote あり、S2・S3 はなし。S1 の「あなた」は hanako。
const (
	store1 port.StoreID = "S1"
	store2 port.StoreID = "S2"
	store3 port.StoreID = "S3"

	mobileWS      workspace.ID = "01TESTWS000000000000000002"
	mobileProject project.ID   = "01TESTPR000000000000000002"
	mobileEpic    item.ID      = "01TESTIT000000000000000001"
	teamWS        workspace.ID = "01TESTWS000000000000000003"
	teamProject   project.ID   = "01TESTPR000000000000000003"
	teamEpic      item.ID      = "01TESTIT000000000000000002"
	archiveWS     workspace.ID = "01TESTWS000000000000000004"
	archiveProj   project.ID   = "01TESTPR000000000000000004"
	archiveEpic   item.ID      = "01TESTIT000000000000000003"
)

type activity struct {
	Store   port.StoreID
	Summary string
}

// recorder は、Write に渡したアクティビティの 1 行を覚える(複数の goroutine から書いてよい)。
type recorder struct {
	*apptest.Stores
	mu  sync.Mutex
	got []activity
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.mu.Lock()
	r.got = append(r.got, activity{id, op.Summary})
	r.mu.Unlock()
	return r.Stores.Write(ctx, id, op, fn)
}

// summaries は、Store stores(なければすべての Store)に書いたアクティビティの文を、書いた順に返す。
func (r *recorder) summaries(stores ...port.StoreID) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, a := range r.got {
		if len(stores) == 0 || slices.Contains(stores, a.Store) {
			out = append(out, a.Summary)
		}
	}
	return out
}

// stubDelivery は Delivery の偽物。reasons に理由を置いた Store は、保存を捨てて理由を返す(送れなかった)。
// gate は Deliver の最初に、inside は移動先の Store のロックを持った状態で fn を呼ぶ前に、呼ぶ(向きの逆の 2 つの移動を重ねるため)。
// 偽の Stores は Store ごとの本物のロックを持つので、fn が別の Store を読むと、重なった相手のロックを待って止まる。
type stubDelivery struct {
	stores  port.Stores
	reasons map[port.StoreID]syncstatus.Reason
	gate    func(port.StoreID)
	inside  func(port.StoreID)
}

var errNotSent = errors.New("not sent")

func (d *stubDelivery) Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) {
	if d.gate != nil {
		d.gate(id)
	}
	r := d.reasons[id]
	err := d.stores.Write(ctx, id, op, func(tx port.Tx) error {
		if d.inside != nil {
			d.inside(id)
		}
		if err := fn(tx); err != nil {
			return err
		}
		if r != "" {
			return errNotSent // fn の保存を残さない
		}
		return nil
	})
	if errors.Is(err, errNotSent) {
		return r, nil
	}
	return "", err
}

type fixture struct {
	svc     transfers.Service
	rec     *recorder
	list    *apptest.StoreList
	deliver *stubDelivery
}

func newFixture(t *testing.T) fixture { return newFixtureWith(t, nil) }

// newFixtureWith は、mobile で Mobile の設定を変えた(例: ステータスを減らす、優先度の名前を変える)フィクスチャ。
func newFixtureWith(t *testing.T, mobile func(*workspace.Settings)) fixture {
	t.Helper()
	ids := apptest.NewIDs()
	stores := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	rec := &recorder{Stores: stores}
	for _, id := range []port.StoreID{store1, store2, store3} {
		stores.AddStore(id)
	}
	if err := apptest.Sample(t.Context(), stores, store1); err != nil {
		t.Fatal(err)
	}
	addWorkspace(t, rec, store1, workspace.Workspace{ID: mobileWS, Name: "Mobile", Prefix: "MB"}, mobileProject, mobileEpic, mobile)
	addMembers(t, rec, store2, "Taro") // 大文字小文字だけが違う綴り
	addWorkspace(t, rec, store2, workspace.Workspace{ID: teamWS, Name: "Team", Prefix: "TM"}, teamProject, teamEpic, nil)
	addMembers(t, rec, store3, "taro")
	addWorkspace(t, rec, store3, workspace.Workspace{ID: archiveWS, Name: "Archive", Prefix: "AR"}, archiveProj, archiveEpic, nil)
	list := &apptest.StoreList{Stores: []port.StoreInfo{
		{ID: store1, Name: "main"}, {ID: store2, Name: "team"}, {ID: store3, Name: "readonly", ReadOnly: true},
	}}
	deliver := &stubDelivery{stores: rec}
	svc := transfers.Service{
		Stores: rec, StoreList: list, Delivery: deliver, Identity: apptest.Identity{Login: "hanako"},
		Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)),
		SyncTargets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: store1, Name: "main", Remote: "https://example.invalid/main.git"}, {ID: store2, Name: "team"}, {ID: store3, Name: "readonly", ReadOnly: true},
		}},
	}
	return fixture{svc: svc, rec: rec, list: list, deliver: deliver}
}

// write は、taro の操作として 1 回保存する。
func write(t *testing.T, s port.Stores, store port.StoreID, fn func(tx port.Tx) error) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	if err := s.Write(t.Context(), store, op, fn); err != nil {
		t.Fatal(err)
	}
}

func addMembers(t *testing.T, s port.Stores, store port.StoreID, logins ...string) {
	t.Helper()
	write(t, s, store, func(tx port.Tx) error {
		ds := []change.Draft{member.Init()}
		for _, l := range logins {
			ds = append(ds, member.Join(l))
		}
		return tx.WriteMembers().Append(ds...)
	})
}

// addWorkspace は、ワークスペースと、プロジェクト 1 つ(「アプリ」・番号 1)と Epic 1 つ(「iOS」・番号 1)を足す。
// pr が空ならワークスペースだけ。値の内部 ID は ws の接頭辞ごとに別(例 VALMB001)。mutate があれば設定を変えてから作る。
func addWorkspace(t *testing.T, s port.Stores, store port.StoreID, ws workspace.Workspace, pr project.ID, epic item.ID, mutate func(*workspace.Settings)) {
	t.Helper()
	n := 0
	st := workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("VAL%s%03d", ws.Prefix, n)) })
	if mutate != nil {
		mutate(&st)
	}
	d, err := workspace.CreateWithSettings(ws, st)
	if err != nil {
		t.Fatal(err)
	}
	write(t, s, store, func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(ws.ID, d); err != nil || pr == "" {
			return err
		}
		p := project.Project{ID: pr, Place: project.Place{Workspace: ws.ID, Number: 1}, Rank: "V", Title: "アプリ", Status: project.Todo}
		e := item.Item{ID: epic, Place: item.Place{Workspace: ws.ID, Project: pr, Number: 1}, Kind: item.Epic, Rank: "V", Title: "iOS"}
		return errors.Join(
			tx.WriteWorkspaces().Append(ws.ID, numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1)),
			tx.WriteProjects().Append(pr, project.Create(p)),
			tx.WriteItems().Append(epic, item.Create(e)),
		)
	})
}

// addItem は、main(S1)にアイテムを作る(作成の行と、番号の行)。
func addItem(t *testing.T, s port.Stores, it item.Item) item.Item {
	t.Helper()
	write(t, s, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(it.ID, item.Create(it)),
			tx.WriteWorkspaces().Append(it.Place.Workspace, numbering.Draft(numbering.Items, it.Place.Number)),
		)
	})
	return it
}

// itemOf は、Store のアイテム(削除済みを含む)。
func itemOf(t *testing.T, s port.Stores, store port.StoreID, id item.ID) item.Item {
	t.Helper()
	var it item.Item
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) { it, err = r.Items().Get(id); return }); err != nil {
		t.Fatal(err)
	}
	return it
}

func logOf(t *testing.T, s port.Stores, store port.StoreID, ref change.Ref) []change.Record {
	t.Helper()
	var log []change.Record
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) { log, err = r.Changes().Of(ref); return }); err != nil {
		t.Fatal(err)
	}
	return log
}

func recordsOf(t *testing.T, s port.Stores, store port.StoreID) []transfer.Record {
	t.Helper()
	var rs []transfer.Record
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) { rs, err = r.Transfers().List(); return }); err != nil {
		t.Fatal(err)
	}
	return rs
}

// nextNumber は、ワークスペースの次の番号。
func nextNumber(t *testing.T, s port.Stores, store port.StoreID, ws workspace.ID) int {
	t.Helper()
	return numbering.Next(logOf(t, s, store, change.Ref{Kind: change.KindWorkspace, ID: ws}), numbering.Items)
}

// valueID は、main(S1)のワークスペース Mobile のステータス(kind が "status")か優先度("priority")の、名前の値の内部 ID。
func valueID(t *testing.T, s port.Stores, kind, name string) change.ID {
	t.Helper()
	var st workspace.Settings
	if err := s.Read(t.Context(), store1, func(r port.Reader) (err error) { st, err = r.Workspaces().Settings(mobileWS); return }); err != nil {
		t.Fatal(err)
	}
	vs := st.Statuses
	if kind == "priority" {
		vs = st.Priorities
	}
	for _, v := range vs {
		if v.Name == name {
			return v.ID
		}
	}
	t.Fatalf("%s %q がない", kind, name)
	return ""
}

func ptr(id item.ID) *item.ID { return &id }

// addRelation は、main(S1)のアイテム from に、to との関連 typ を足す(持つのは from)。
func addRelation(t *testing.T, s port.Stores, from, to item.ID, typ string) {
	t.Helper()
	write(t, s, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(from, change.NewAdd("relations", item.Relation{Type: typ, To: to}))
	})
}

// newBarrier は、n 回目に呼ばれるまで、呼んだものをすべて待たせる。そろったあとの呼び出しは待たない(1 回きり)。
func newBarrier(n int) func(port.StoreID) {
	var mu sync.Mutex
	arrived := 0
	all := make(chan struct{})
	return func(port.StoreID) {
		mu.Lock()
		arrived++
		if arrived == n {
			close(all)
		}
		mu.Unlock()
		<-all
	}
}
