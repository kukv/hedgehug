package transfers_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/transfers"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	apptransfers "github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// env は、S1「main」(Sample と SampleMobile。remote あり)と S2「team」(SampleTeam。remote なし)の API。
// アイテムの API も登録する(移したあとのアイテムを読むため)。
// failS1 を true にすると、S1 への保存(移動元の保存)が失敗する。deliver.undo に理由を置くと、2 回目の Deliver
// (移動先の保存の取り消し)が送れない。
type env struct {
	api     humatest.TestAPI
	stores  *apptest.Stores
	list    *apptest.StoreList
	reasons map[port.StoreID]syncstatus.Reason
	failS1  bool
	deliver *deliver
	targets *apptest.SyncTargets
}

// failing は、*fail が true のあいだ、Store S1 への Write を失敗させる。
type failing struct {
	port.Stores
	fail *bool
}

var errBoom = errors.New("boom")

func (f failing) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if *f.fail && id == "S1" {
		return errBoom
	}
	return f.Stores.Write(ctx, id, op, fn)
}

// hide は、Store id をサイドバーで非表示にする(StoreList から除く)。
func (e *env) hide(id port.StoreID) {
	e.list.Stores = slices.DeleteFunc(e.list.Stores, func(i port.StoreInfo) bool { return i.ID == id })
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	s.AddStore("S2")
	if err := errors.Join(apptest.Sample(t.Context(), s, "S1"), apptest.SampleMobile(t.Context(), s, "S1"), apptest.SampleTeam(t.Context(), s, "S2")); err != nil {
		t.Fatal(err)
	}
	e := &env{
		stores: s, reasons: map[port.StoreID]syncstatus.Reason{},
		list: &apptest.StoreList{Stores: []port.StoreInfo{{ID: "S1", Name: "main"}, {ID: "S2", Name: "team"}}},
	}
	e.targets = &apptest.SyncTargets{Targets: []port.SyncTarget{
		{ID: "S1", Name: "main", Remote: "https://example.invalid/main.git"}, {ID: "S2", Name: "team"},
	}}
	_, e.api = humatest.New(t)
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	me := apptest.Identity{Login: "taro"}
	e.deliver = &deliver{stores: s, reasons: e.reasons}
	transfers.Register(e.api, apptransfers.Service{
		Stores: failing{Stores: s, fail: &e.failS1}, Delivery: e.deliver, Identity: me, Clock: clock, IDs: ids,
		StoreList:   e.list,
		SyncTargets: e.targets,
	})
	items.Register(e.api, appitems.Service{Stores: s, IDs: ids, Identity: me, Clock: clock})
	return e
}

// deliver は Delivery の偽物。reasons に理由を置いた Store は、保存を捨てて理由を返す(送れなかった)。
type deliver struct {
	stores  port.Stores
	reasons map[port.StoreID]syncstatus.Reason
	undo    syncstatus.Reason
	calls   int
}

var errNotSent = errors.New("not sent")

func (d *deliver) Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) {
	d.calls++
	if d.calls == 2 && d.undo != "" {
		return d.undo, nil
	}
	r := d.reasons[id]
	err := d.stores.Write(ctx, id, op, func(tx port.Tx) error {
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

func TestTargets(t *testing.T) {
	e := newEnv(t)
	res := e.api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/transfer-targets")
	var got []struct {
		StoreID, StoreName, WorkspaceID, WorkspaceName, Prefix string
		Projects                                               []struct {
			ID, Title string
			Epics     []struct{ ID, Display, Title string }
		}
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil || len(got) != 2 {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	m, tm := got[0], got[1]
	if m.StoreID != "S1" || m.StoreName != "main" || m.WorkspaceID != string(apptest.SampleMobileWorkspace) || m.WorkspaceName != "Mobile" || m.Prefix != "MB" ||
		m.Projects[0].ID != string(apptest.SampleMobileProject) || m.Projects[0].Title != "アプリ" ||
		m.Projects[0].Epics[0].ID != string(apptest.SampleMobileEpic) || m.Projects[0].Epics[0].Display != "MB-1" || m.Projects[0].Epics[0].Title != "iOS" ||
		tm.StoreName != "team" || tm.Prefix != "TM" {
		t.Errorf("targets = %+v", got)
	}
	if res := e.api.Get("/api/stores/S1/items/01NOSUCHITEM0000000000000/transfer-targets"); res.Code != 404 {
		t.Errorf("ないアイテム: %d", res.Code)
	}
}

func TestTargetsFromAnotherStore(t *testing.T) {
	e := newEnv(t)
	res := e.api.Get("/api/stores/S2/items/" + string(apptest.SampleTeamEpic) + "/transfer-targets")
	if res.Code != 200 || !strings.HasPrefix(res.Body.String(), `[{"storeId":"S1"`) {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	if !strings.Contains(res.Body.String(), `"epics":[{"id":"`+string(apptest.SampleEpic)+`"`) {
		t.Errorf("%s", res.Body)
	}
}

func TestPreview(t *testing.T) {
	e := newEnv(t)
	q := "?toStore=S1&toWorkspace=" + string(apptest.SampleMobileWorkspace) + "&toProject=" + string(apptest.SampleMobileProject)
	res := e.api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/transfer-preview" + q)
	want := `{"below":0,"deleted":0,"nextDisplay":"MB-2","crossStore":false,"remoteNotice":false,"destinationHasRemote":true,"replacements":[]}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Errorf("%d %s", res.Code, res.Body)
	}
	q = "?toStore=S2&toWorkspace=" + string(apptest.SampleTeamWorkspace) + "&toProject=" + string(apptest.SampleTeamProject)
	res = e.api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/transfer-preview" + q)
	if !strings.Contains(res.Body.String(), `"nextDisplay":"TM-2","crossStore":true,"remoteNotice":true,"destinationHasRemote":false`) {
		t.Errorf("別の Store: %s", res.Body)
	}
	if res := e.api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/transfer-preview?toStore=S9&toWorkspace=x&toProject=y"); res.Code != 404 {
		t.Errorf("ない Store: %d", res.Code)
	}
}

// 移動先にない名前のステータスは「ステータスの置き換え」の行になる。
func TestPreviewReplacements(t *testing.T) {
	e := newEnv(t)
	todo := workspace.List{Kind: workspace.ListStatuses}.Field("01SAMPLETMV000000000000001", "name")
	op := port.Operation{Actor: change.User("taro"), Summary: "ステータスの名前を変更"}
	if err := e.stores.Write(t.Context(), "S2", op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleTeamWorkspace, change.NewSet(todo, "Todo", "未着手"))
	}); err != nil {
		t.Fatal(err)
	}
	q := "?toStore=S2&toWorkspace=" + string(apptest.SampleTeamWorkspace) + "&toProject=" + string(apptest.SampleTeamProject)
	res := e.api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/transfer-preview" + q)
	want := `"replacements":[{"status":"` + string(apptest.SampleStatusTodo) + `","name":"Todo","count":1,"choices":[{"id":"01SAMPLETMV000000000000001","name":"未着手"},`
	if res.Code != 200 || !strings.Contains(res.Body.String(), want) || !strings.Contains(res.Body.String(), `"default":"01SAMPLETMV000000000000001"}]`) {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}
