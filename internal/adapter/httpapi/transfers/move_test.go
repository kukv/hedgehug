package transfers_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

func body(store, ws, pr, parent string) map[string]any {
	b := map[string]any{"toStore": store, "toWorkspace": ws, "toProject": pr, "statusChoices": map[string]string{}}
	if parent != "" {
		b["toParent"] = parent
	}
	return b
}

func TestTransferWithinTheStoreAndAcrossStores(t *testing.T) {
	e := newEnv(t)
	task := "/api/stores/S1/items/" + string(apptest.SampleTask)
	res := e.api.Post(task+"/transfer", body("S1", string(apptest.SampleMobileWorkspace), string(apptest.SampleMobileProject), string(apptest.SampleMobileEpic)))
	want := `{"toStore":"S1","itemId":"` + string(apptest.SampleTask) + `","display":"MB-2","emptied":[]}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Fatalf("同じ Store: %d %s", res.Code, res.Body)
	}
	res = e.api.Post(task+"/transfer", body("S2", string(apptest.SampleTeamWorkspace), string(apptest.SampleTeamProject), string(apptest.SampleTeamEpic)))
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"toStore":"S2"`) || !strings.Contains(res.Body.String(), `"display":"TM-2"`) {
		t.Fatalf("別の Store: %d %s", res.Code, res.Body)
	}
	if res := e.api.Get("/api/stores/S2/items/" + string(apptest.SampleTask)); res.Code != 200 {
		t.Errorf("移動先で読めない: %d", res.Code)
	}
	if res := e.api.Get(task); res.Code != 404 {
		t.Errorf("移動元に残っている: %d", res.Code)
	}
}

// 送れなかったとき: 422。文は「{Store 名} に送信できないため移動できません: {理由}」。移動元はそのまま。
func TestTransferThatCannotBeSent(t *testing.T) {
	e := newEnv(t)
	e.reasons["S2"] = syncstatus.WriteDenied
	task := "/api/stores/S1/items/" + string(apptest.SampleTask)
	res := e.api.Post(task+"/transfer", body("S2", string(apptest.SampleTeamWorkspace), string(apptest.SampleTeamProject), string(apptest.SampleTeamEpic)))
	if res.Code != 422 || !strings.Contains(res.Body.String(), `"detail":"team に送信できないため移動できません: remote への書き込みが許可されていません"`) {
		t.Errorf("%d %s", res.Code, res.Body)
	}
	if res := e.api.Get(task); res.Code != 200 || !strings.Contains(res.Body.String(), `"workspace":"`+string(apptest.SampleWorkspace)+`"`) {
		t.Errorf("移動元が変わった: %d %s", res.Code, res.Body)
	}
}

func TestTransferNotFound(t *testing.T) {
	e := newEnv(t)
	epic := "/api/stores/S1/items/" + string(apptest.SampleEpic)
	if res := e.api.Post(epic+"/transfer", body("S1", string(apptest.SampleMobileWorkspace), string(apptest.SampleMobileProject), string(apptest.SampleMobileEpic))); res.Code != 404 {
		t.Errorf("Epic に親: %d", res.Code)
	}
	if res := e.api.Post(epic+"/transfer", body("S9", string(apptest.SampleMobileWorkspace), string(apptest.SampleMobileProject), "")); res.Code != 404 {
		t.Errorf("ない Store: %d", res.Code)
	}
}
