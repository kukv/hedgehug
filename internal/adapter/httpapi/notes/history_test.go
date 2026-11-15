package notes_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	apphistory "github.com/kukv/hedgehug/internal/app/history"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// receivedAll は apphistory.ReceivedOps の偽物。尋ねられた op をすべて、同期で受け取った保存のものとして返す。
type receivedAll struct{}

func (receivedAll) Received(_ context.Context, _ port.StoreID, ops []change.ID) (map[change.ID]bool, error) {
	out := map[change.ID]bool{}
	for _, op := range ops {
		out[op] = true
	}
	return out, nil
}

func newHistoryAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	return newHistoryAPIWith(t, nil)
}

func newHistoryAPIWith(t *testing.T, received apphistory.ReceivedOps) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("hanako"), Summary: "タイトルを変更"}
	err := s.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン"))
	})
	if err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{History: apphistory.Service{Stores: s, StoreList: apptest.StoreList{Stores: []port.StoreInfo{{ID: "S1", Name: "main"}}}, Clock: clock, Received: received}})
	return api
}

func TestItemHistory(t *testing.T) {
	api := newHistoryAPI(t)
	res := api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/history")
	body := res.Body.String()
	if res.Code != http.StatusOK {
		t.Fatalf("%d %s", res.Code, body)
	}
	newer, older := strings.Index(body, `"text":"タイトルを「ログイン」に変更"`), strings.Index(body, `"text":"作成"`)
	if newer < 0 || older < 0 || newer > older {
		t.Errorf("新しい順に、タイトルの変更、作成: %s", body)
	}
	for _, want := range []string{`"who":"hanako"`, `"who":"taro"`, `"at":"2026-11-13T`, `"received":false`} {
		if !strings.Contains(body, want) {
			t.Errorf("lacks %s: %s", want, body)
		}
	}
}

func TestProjectHistoryAndNotFound(t *testing.T) {
	api := newHistoryAPI(t)
	res := api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject) + "/history")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"text":"作成"`) {
		t.Errorf("project: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/01NOPE00000000000000000000/history"); res.Code != http.StatusNotFound {
		t.Errorf("no such item: %d", res.Code)
	}
	if res := api.Get("/api/stores/S1/projects/01NOPE00000000000000000000/history"); res.Code != http.StatusNotFound {
		t.Errorf("no such project: %d", res.Code)
	}
}

// 同期で受け取った保存の行は received が true(C4d-16)。
func TestHistoryReceived(t *testing.T) {
	api := newHistoryAPIWith(t, receivedAll{})
	for _, path := range []string{"/items/" + string(apptest.SampleTask), "/projects/" + string(apptest.SampleProject)} {
		res := api.Get("/api/stores/S1" + path + "/history")
		body := res.Body.String()
		if res.Code != http.StatusOK || !strings.Contains(body, `"received":true`) || strings.Contains(body, `"received":false`) {
			t.Errorf("%s: %d %s", path, res.Code, body)
		}
	}
}
