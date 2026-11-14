package transfers_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

const webWorkspace = "/api/stores/S1/workspaces/" + string(apptest.SampleWorkspace)

// 移せる Store の候補。0 件のときは targets が [] で hiddenOnly が真偽値で返る。ないワークスペースは 404。
func TestWorkspaceTransferTargets(t *testing.T) {
	e := newEnv(t)
	res := e.api.Get(webWorkspace + "/transfer-targets")
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"targets":[{"storeId":"S2","storeName":"team"}],"hiddenOnly":false}` {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	e.hide("S1")
	res = e.api.Get("/api/stores/S2/workspaces/" + string(apptest.SampleTeamWorkspace) + "/transfer-targets")
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"targets":[],"hiddenOnly":false}` {
		t.Errorf("0 件: %d %s", res.Code, res.Body)
	}
	e.targets.Targets[0].Hidden = true
	res = e.api.Get("/api/stores/S2/workspaces/" + string(apptest.SampleTeamWorkspace) + "/transfer-targets")
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"targets":[],"hiddenOnly":true}` {
		t.Errorf("非表示の Store だけ: %d %s", res.Code, res.Body)
	}
	if res := e.api.Get("/api/stores/S1/workspaces/01NOSUCHWORKSPACE000000/transfer-targets"); res.Code != 404 {
		t.Errorf("ないワークスペース: %d", res.Code)
	}
}

// 予告の形。空の配列は null でなく []。移動先に同じ接頭辞があれば 422 で文がそのまま返る。
func TestWorkspaceTransferPreview(t *testing.T) {
	e := newEnv(t)
	res := e.api.Get(webWorkspace + "/transfer-preview?toStore=S2")
	var got struct {
		Source, Destination                string
		Projects, Items, Deleted           int
		People                             []struct{ Login, Name string }
		RemoteNotice, DestinationHasRemote bool
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	if got.Source != "main" || got.Destination != "team" || got.Projects != 1 || got.Items == 0 || !got.RemoteNotice || got.DestinationHasRemote {
		t.Errorf("preview = %+v", got)
	}
	if strings.Contains(res.Body.String(), "null") || !strings.Contains(res.Body.String(), `"people":[`) {
		t.Errorf("空の配列は []: %s", res.Body)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "接頭辞を変更"}
	if err := e.stores.Write(t.Context(), "S2", op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleTeamWorkspace, change.NewSet("prefix", "TM", "HH"))
	}); err != nil {
		t.Fatal(err)
	}
	res = e.api.Get(webWorkspace + "/transfer-preview?toStore=S2")
	if res.Code != 422 || !strings.Contains(res.Body.String(), `"detail":"team: この Store には同じ接頭辞のワークスペースがあります"`) {
		t.Errorf("断り: %d %s", res.Code, res.Body)
	}
	if res := e.api.Get(webWorkspace + "/transfer-preview?toStore=S9"); res.Code != 404 {
		t.Errorf("ない Store: %d", res.Code)
	}
}

// 移動の応答。ないとき 404。
func TestTransferWorkspace(t *testing.T) {
	e := newEnv(t)
	if res := e.api.Post(webWorkspace+"/transfer", map[string]any{"toStore": "S9"}); res.Code != 404 {
		t.Errorf("ない Store: %d", res.Code)
	}
	res := e.api.Post(webWorkspace+"/transfer", map[string]any{"toStore": "S2"})
	want := `{"toStore":"S2","toStoreName":"team","workspaceId":"` + string(apptest.SampleWorkspace) + `","prefix":"HH"}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	if res := e.api.Post(webWorkspace+"/transfer", map[string]any{"toStore": "S2"}); res.Code != 404 {
		t.Errorf("移したあと: %d", res.Code)
	}
}

// 送れなかったとき: 422。detail は文の全文。
func TestTransferWorkspaceThatCannotBeSent(t *testing.T) {
	e := newEnv(t)
	e.reasons["S2"] = syncstatus.WriteDenied
	res := e.api.Post(webWorkspace+"/transfer", map[string]any{"toStore": "S2"})
	if res.Code != 422 || !strings.Contains(res.Body.String(), `"detail":"team に送信できないため移動できません: remote への書き込みが許可されていません"`) {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}

// 移動元の保存が失敗し、取り消しも送れなかったとき: 409。
func TestTransferWorkspaceWhoseUndoFailed(t *testing.T) {
	e := newEnv(t)
	e.failS1 = true
	e.deliver.undo = syncstatus.Unreachable
	res := e.api.Post(webWorkspace+"/transfer", map[string]any{"toStore": "S2"})
	if res.Code != 409 || !strings.Contains(res.Body.String(), `"detail":"ワークスペース「Web」を team へ移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください"`) {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}

// 画面のアドレスの接頭辞。いまの場所・移した先(movedFrom)・表示中でない Store(gone)・見つからない(404)。
func TestLocateWorkspaceAPI(t *testing.T) {
	e := newEnv(t)
	res := e.api.Get("/api/locate-workspace?store=main&prefix=HH")
	want := `{"kind":"found","storeId":"S1","workspaceId":"` + string(apptest.SampleWorkspace) + `"}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Fatalf("いまの接頭辞: %d %s", res.Code, res.Body)
	}
	if res := e.api.Post(webWorkspace+"/transfer", map[string]any{"toStore": "S2"}); res.Code != 200 {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	res = e.api.Get("/api/locate-workspace?store=main&prefix=HH")
	var got struct {
		Kind, StoreID, WorkspaceID string
		MovedFrom                  struct{ Old, At, Display, Current string }
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil ||
		got.Kind != "found" || got.StoreID != "S2" || got.WorkspaceID != string(apptest.SampleWorkspace) ||
		got.MovedFrom.Old != "HH" || got.MovedFrom.Display != "team" || got.MovedFrom.Current != "team" || !strings.HasPrefix(got.MovedFrom.At, "2026-11-13T09:00:") {
		t.Errorf("旧アドレス: %d %s", res.Code, res.Body)
	}
	e.hide("S2")
	res = e.api.Get("/api/locate-workspace?store=main&prefix=HH")
	var gone struct{ Kind, Reason, Old, At, NewDisplay, StoreName string }
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &gone) != nil ||
		gone.Kind != "gone" || gone.Reason != "hidden" || gone.Old != "HH" || gone.NewDisplay != "team" || gone.StoreName != "team" || !strings.HasPrefix(gone.At, "2026-11-13T09:00:") {
		t.Errorf("gone: %d %s", res.Code, res.Body)
	}
	if res := e.api.Get("/api/locate-workspace?store=main&prefix=ZZ"); res.Code != 404 {
		t.Errorf("見つからない: %d", res.Code)
	}
}
