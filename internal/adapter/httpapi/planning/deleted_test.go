package planning_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// send は、組み立てた本物のサーバーへ要求を送り、状態と Hedgehug-Restored と本文を返す。
func send(t *testing.T, srv *httptest.Server, method, path string, body any) (int, []string, []byte) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, res.Header.Values("Hedgehug-Restored"), out
}

// newServer は、サンプルを書いた Store S1 の、組み立てた本物のサーバーと、その Store。今日は 2026-11-13。
func newServer(t *testing.T) (*httptest.Server, *apptest.Stores) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.New("secret", fstest.MapFS{}, httpapi.Deps{
		Version: "test", Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}, Clock: clock,
	}))
	t.Cleanup(srv.Close)
	return srv, s
}

// idIn は、本文の id。
func idIn(t *testing.T, code int, body []byte) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.ID == "" {
		t.Fatalf("body: %v %d %s", err, code, body)
	}
	return v.ID
}

// 別のタブで削除された(root: true)アイテムのスプリントを変えて保存すると、削除を取り消し、応答に表示 ID を
// 付ける(C3a-screen S1)。同じ値で断られたとき(404)は付けない。
func TestSetSprintOfDeletedItemSetsRestoredHeader(t *testing.T) {
	srv, s := newServer(t)
	code, _, body := send(t, srv, http.MethodPost, "/api/stores/S1/items",
		map[string]any{"project": string(apptest.SampleProject), "parent": string(apptest.SampleEpic), "kind": "Task", "title": "API"})
	id := idIn(t, code, body)
	code, _, body = send(t, srv, http.MethodPost, base+"/sprints", map[string]any{"name": "Sprint 1", "start": "", "end": "", "goal": ""})
	sp := idIn(t, code, body)
	if err := apptest.DeleteItems(t.Context(), s, "S1", item.ID(id)); err != nil {
		t.Fatal(err)
	}
	if code, got, _ := send(t, srv, http.MethodPut, itemBase+id+"/sprint", map[string]any{"sprint": nil}); code != http.StatusNotFound || len(got) != 0 {
		t.Errorf("same sprint: %d %v", code, got)
	}
	if code, got, _ := send(t, srv, http.MethodPut, itemBase+id+"/sprint", map[string]any{"sprint": sp}); code != http.StatusNoContent || !slices.Equal(got, []string{"HH-3"}) {
		t.Errorf("sprint: %d %v", code, got)
	}
}

// 削除済みのワークスペースのスプリントの復元は、ワークスペースの復元を先に求めて断る(08 356 行)。
// スプリントの読み取りは、ワークスペースがないときと同じ 404(B5b-4)。
func TestRestoreSprintUnderDeletedWorkspace(t *testing.T) {
	srv, s := newServer(t)
	code, _, body := send(t, srv, http.MethodPost, base+"/sprints", map[string]any{"name": "Sprint 1", "start": "", "end": "", "goal": ""})
	sp := idIn(t, code, body)
	if code, _, body := send(t, srv, http.MethodDelete, base+"/sprints/"+sp, nil); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, body)
	}
	if err := apptest.DeleteWorkspace(t.Context(), s, "S1", apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	code, _, body = send(t, srv, http.MethodPost, base+"/sprints/"+sp+"/restore", nil)
	if code != http.StatusUnprocessableEntity || !bytes.Contains(body, []byte("ワークスペースも削除されています。先にワークスペースを復元してください")) {
		t.Errorf("restore: %d %s", code, body)
	}
	if code, _, body := send(t, srv, http.MethodGet, base+"/sprints", nil); code != http.StatusNotFound {
		t.Errorf("cards: %d %s", code, body)
	}
}
