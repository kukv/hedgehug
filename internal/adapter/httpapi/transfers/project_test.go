package transfers_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

func projectBody(store, ws string) map[string]any {
	return map[string]any{"toStore": store, "toWorkspace": ws, "statusChoices": map[string]string{}}
}

const mainProject = "/api/stores/S1/projects/" + string(apptest.SampleProject)

// 候補は、自分のワークスペースを除いた Store・ワークスペースの並び。ないプロジェクトは 404。
func TestProjectTransferTargetsAPI(t *testing.T) {
	e := newEnv(t)
	res := e.api.Get(mainProject + "/transfer-targets")
	var got []struct{ StoreID, StoreName, WorkspaceID, WorkspaceName string }
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil || len(got) != 2 {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	if got[0].StoreID != "S1" || got[0].StoreName != "main" || got[0].WorkspaceID != string(apptest.SampleMobileWorkspace) || got[0].WorkspaceName != "Mobile" ||
		got[1].StoreID != "S2" || got[1].StoreName != "team" || got[1].WorkspaceID != string(apptest.SampleTeamWorkspace) {
		t.Errorf("targets = %+v", got)
	}
	if res := e.api.Get("/api/stores/S1/projects/01NOSUCHPROJECT00000000000/transfer-targets"); res.Code != 404 {
		t.Errorf("ないプロジェクト: %d", res.Code)
	}
}

// 予告の形。空の配列は null でなく [] になる。
func TestProjectTransferPreviewAPI(t *testing.T) {
	e := newEnv(t)
	q := "?toStore=S1&toWorkspace=" + string(apptest.SampleMobileWorkspace)
	res := e.api.Get(mainProject + "/transfer-preview" + q)
	var got map[string]any
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	for _, k := range []string{"items", "deleted", "prefix", "source", "destination", "crossStore", "remoteNotice", "destinationHasRemote", "hasRules"} {
		if _, ok := got[k]; !ok {
			t.Errorf("%s がない: %s", k, res.Body)
		}
	}
	if got["prefix"] != "MB" || got["crossStore"] != false {
		t.Errorf("同じ Store: %s", res.Body)
	}
	if !strings.Contains(res.Body.String(), `"notCarried":[]`) {
		t.Errorf("notCarried は []: %s", res.Body)
	}
	res = e.api.Get(mainProject + "/transfer-preview?toStore=S2&toWorkspace=" + string(apptest.SampleTeamWorkspace))
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"prefix":"TM"`) || !strings.Contains(res.Body.String(), `"crossStore":true`) ||
		strings.Contains(res.Body.String(), "null") {
		t.Errorf("別の Store: %d %s", res.Code, res.Body)
	}
	if res := e.api.Get(mainProject + "/transfer-preview?toStore=S9&toWorkspace=x"); res.Code != 404 {
		t.Errorf("ない Store: %d", res.Code)
	}
}

// 移動の応答。同じ Store・別の Store。
func TestTransferProjectAPI(t *testing.T) {
	e := newEnv(t)
	res := e.api.Post(mainProject+"/transfer", projectBody("S1", string(apptest.SampleMobileWorkspace)))
	var got struct{ ToStore, ProjectID, Display, Destination string }
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil ||
		got.ToStore != "S1" || got.ProjectID != string(apptest.SampleProject) || got.Display != "MB-P2" || got.Destination == "" {
		t.Fatalf("同じ Store: %d %s", res.Code, res.Body)
	}
	res = e.api.Post(mainProject+"/transfer", projectBody("S2", string(apptest.SampleTeamWorkspace)))
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"toStore":"S2"`) || !strings.Contains(res.Body.String(), `"display":"TM-P2"`) {
		t.Fatalf("別の Store: %d %s", res.Code, res.Body)
	}
	if res := e.api.Post(mainProject+"/transfer", projectBody("S9", "x")); res.Code != 404 {
		t.Errorf("ない Store: %d", res.Code)
	}
}

// 送れなかったとき: 422。detail は文の全文。
func TestTransferProjectThatCannotBeSent(t *testing.T) {
	e := newEnv(t)
	e.reasons["S2"] = syncstatus.WriteDenied
	res := e.api.Post(mainProject+"/transfer", projectBody("S2", string(apptest.SampleTeamWorkspace)))
	if res.Code != 422 || !strings.Contains(res.Body.String(), `"detail":"team に送信できないため移動できません: remote への書き込みが許可されていません"`) {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}

// 移動元の保存が失敗し、取り消しも送れなかったとき: 409。
func TestTransferProjectWhoseUndoFailed(t *testing.T) {
	e := newEnv(t)
	e.failS1 = true
	e.deliver.undo = syncstatus.Unreachable
	res := e.api.Post(mainProject+"/transfer", projectBody("S2", string(apptest.SampleTeamWorkspace)))
	if res.Code != 409 || !strings.Contains(res.Body.String(), "取り消しもできませんでした") {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}

func TestLocateProjectAPI(t *testing.T) {
	e := newEnv(t)
	res := e.api.Get("/api/locate-project?store=main&id=HH-P1")
	want := `{"kind":"found","storeId":"S1","projectId":"` + string(apptest.SampleProject) + `"}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Fatalf("いまの ID: %d %s", res.Code, res.Body)
	}
	e.api.Post(mainProject+"/transfer", projectBody("S2", string(apptest.SampleTeamWorkspace)))
	res = e.api.Get("/api/locate-project?store=main&id=HH-P1")
	var got struct {
		Kind, StoreID, ProjectID string
		MovedFrom                struct{ Old, At, Display, Current string }
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil ||
		got.Kind != "found" || got.StoreID != "S2" || got.ProjectID != string(apptest.SampleProject) ||
		got.MovedFrom.Old != "HH-P1" || got.MovedFrom.Display != "TM-P2" || got.MovedFrom.Current != "TM-P2" || !strings.HasPrefix(got.MovedFrom.At, "2026-11-13T09:00:") {
		t.Errorf("旧 ID: %d %s", res.Code, res.Body)
	}
	e.hide("S2")
	res = e.api.Get("/api/locate-project?store=main&id=HH-P1")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"kind":"gone","reason":"hidden","old":"HH-P1"`) || !strings.Contains(res.Body.String(), `"storeName":"team"`) {
		t.Errorf("gone: %d %s", res.Code, res.Body)
	}
	if res := e.api.Get("/api/locate-project?store=main&id=ZZ-P9"); res.Code != 404 {
		t.Errorf("見つからない: %d", res.Code)
	}
	if res := e.api.Get("/api/locate-project?store=main&id=HH-2"); res.Code != 404 {
		t.Errorf("アイテムの形: %d", res.Code)
	}
}
