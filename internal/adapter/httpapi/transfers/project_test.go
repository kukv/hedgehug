package transfers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// renameTeamStatus は、team のワークスペースのステータス(内部 ID の末尾 n)の名前を name に変える。
func renameTeamStatus(t *testing.T, e *env, n, from, name string) {
	t.Helper()
	f := workspace.List{Kind: workspace.ListStatuses}.Field(change.ID("01SAMPLETMV000000000000"+n), "name")
	op := port.Operation{Actor: change.User("taro"), Summary: "ステータスの名前を変更"}
	if err := e.stores.Write(t.Context(), "S2", op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleTeamWorkspace, change.NewSet(f, from, name))
	}); err != nil {
		t.Fatal(err)
	}
}

// 別のタブでルート削除(root: true)されたプロジェクトを移すと、削除を取り消し、本物のミドルウェアを通した応答に
// Hedgehug-Restored で旧 ID(HH-P1)が付く。
func TestTransferProjectDeletedElsewhereSetsRestoredHeader(t *testing.T) {
	for _, to := range []struct{ name, store, ws string }{
		{"同じ Store", "S1", string(apptest.SampleMobileWorkspace)},
		{"別の Store", "S2", string(apptest.SampleTeamWorkspace)},
	} {
		t.Run(to.name, func(t *testing.T) {
			e := newEnv(t)
			op := port.Operation{Actor: change.User("taro"), Summary: "プロジェクトを削除"}
			if err := e.stores.Write(t.Context(), "S1", op, func(tx port.Tx) error {
				return tx.WriteProjects().Append(apptest.SampleProject, change.NewDeleteEntity(true))
			}); err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(httpapi.New("secret", fstest.MapFS{}, httpapi.Deps{
				Version: "test", Stores: e.stores, IDs: apptest.NewIDs(), Identity: apptest.Identity{Login: "taro"},
				Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), StoreList: e.list, Delivery: e.deliver,
				SyncTargets: &apptest.SyncTargets{Targets: []port.SyncTarget{{ID: "S1", Name: "main", Remote: "https://example.invalid/main.git"}, {ID: "S2", Name: "team"}}},
			}))
			t.Cleanup(srv.Close)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+mainProject+"/transfer",
				strings.NewReader(`{"toStore":"`+to.store+`","toWorkspace":"`+to.ws+`","statusChoices":{}}`))
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
			if res.StatusCode != 200 || !slices.Equal(res.Header.Values("Hedgehug-Restored"), []string{"HH-P1"}) {
				t.Errorf("%d %v", res.StatusCode, res.Header)
			}
		})
	}
}

// ルールがあり、移動先にないステータスのときの予告の中身。引き継げないルールの missing は空でも [] になる。
func TestProjectTransferPreviewRowsAPI(t *testing.T) {
	e := newEnv(t)
	renameTeamStatus(t, e, "001", "Todo", "未着手")
	renameTeamStatus(t, e, "004", "Pending", "保留中")
	op := port.Operation{Actor: change.User("taro"), Summary: "ステータスを使わない"}
	if err := e.stores.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewAdd(projectrule.HiddenField(apptest.SampleProject), apptest.SampleStatusPending))
	}); err != nil {
		t.Fatal(err)
	}
	res := e.api.Get(mainProject + "/transfer-preview?toStore=S2&toWorkspace=" + string(apptest.SampleTeamWorkspace))
	var got struct {
		HasRules   bool
		NotCarried []struct {
			Kind, Name string
			Missing    []string
			Count      int
		}
		Replacements []struct {
			Status, Name string
			Count        int
			Choices      []struct{ ID, Name string }
			Default      string
		}
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	if !got.HasRules || len(got.NotCarried) != 1 || got.NotCarried[0].Kind != "hidden" || got.NotCarried[0].Name != "Pending" || got.NotCarried[0].Count != 0 {
		t.Errorf("notCarried = %+v", got.NotCarried)
	}
	if !strings.Contains(res.Body.String(), `"notCarried":[{"kind":"hidden","name":"Pending","missing":[],"count":0}]`) {
		t.Errorf("missing は []: %s", res.Body)
	}
	if len(got.Replacements) != 1 {
		t.Fatalf("replacements = %+v", got.Replacements)
	}
	r := got.Replacements[0]
	if r.Status != string(apptest.SampleStatusTodo) || r.Name != "Todo" || r.Count != 2 || r.Default != "01SAMPLETMV000000000000001" ||
		len(r.Choices) == 0 || r.Choices[0].ID != "01SAMPLETMV000000000000001" || r.Choices[0].Name != "未着手" {
		t.Errorf("replacement = %+v", r)
	}
}

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
	var gone struct{ At, NewDisplay string }
	if json.Unmarshal(res.Body.Bytes(), &gone) != nil || gone.NewDisplay != "TM-P2" || !strings.HasPrefix(gone.At, "2026-11-13T09:00:") {
		t.Errorf("gone の at・newDisplay: %s", res.Body)
	}
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
