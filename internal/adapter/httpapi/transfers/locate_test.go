package transfers_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

func TestLocate(t *testing.T) {
	e := newEnv(t)
	task := string(apptest.SampleTask)
	res := e.api.Get("/api/locate?store=main&id=HH-2") // いまの ID
	want := `{"kind":"found","storeId":"S1","projectId":"` + string(apptest.SampleProject) + `","itemId":"` + task + `"}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Fatalf("いまの ID: %d %s", res.Code, res.Body)
	}
	e.api.Post("/api/stores/S1/items/"+task+"/transfer", body("S2", string(apptest.SampleTeamWorkspace), string(apptest.SampleTeamProject), string(apptest.SampleTeamEpic)))
	res = e.api.Get("/api/locate?store=main&id=HH-2") // 旧 ID は team へ辿り着く
	var got struct {
		Kind, StoreID, ProjectID, ItemID string
		MovedFrom                        struct{ Old, At, Display, Current string }
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil ||
		got.Kind != "found" || got.StoreID != "S2" || got.ItemID != task || got.ProjectID != string(apptest.SampleTeamProject) ||
		got.MovedFrom.Old != "HH-2" || got.MovedFrom.Display != "TM-2" || got.MovedFrom.Current != "TM-2" || !strings.HasPrefix(got.MovedFrom.At, "2026-11-12T09:00:") {
		t.Errorf("旧 ID: %d %s", res.Code, res.Body)
	}
	if err := apptest.DeleteItems(t.Context(), e.stores, "S2", apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	res = e.api.Get("/api/locate?store=main&id=HH-2") // 削除済みは、画面を開かず理由と材料を返す(文は画面が作る)
	var gone struct{ Kind, Reason, Old, At, NewDisplay string }
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &gone) != nil ||
		gone.Kind != "gone" || gone.Reason != "deleted" || gone.Old != "HH-2" || gone.NewDisplay != "TM-2" || !strings.HasPrefix(gone.At, "2026-11-12T09:00:") {
		t.Errorf("削除済み: %d %s", res.Code, res.Body)
	}
	if body := res.Body.String(); strings.Contains(body, "message") || strings.Contains(body, "storeName") || strings.Contains(body, "itemId") {
		t.Errorf("deleted に message・storeName・itemId は出さない: %s", body)
	}
	if res := e.api.Get("/api/locate?store=main&id=ZZ-9"); res.Code != 404 {
		t.Errorf("見つからない: %d", res.Code)
	}
}

// 移動先の Store が表示中でないときは hidden と、その Store 名を返す。
func TestLocateHidden(t *testing.T) {
	e := newEnv(t)
	task := string(apptest.SampleTask)
	e.api.Post("/api/stores/S1/items/"+task+"/transfer", body("S2", string(apptest.SampleTeamWorkspace), string(apptest.SampleTeamProject), string(apptest.SampleTeamEpic)))
	e.hide("S2")
	res := e.api.Get("/api/locate?store=main&id=HH-2")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"kind":"gone","reason":"hidden","old":"HH-2"`) || !strings.Contains(res.Body.String(), `"newDisplay":"TM-2","storeName":"team"}`) {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}
