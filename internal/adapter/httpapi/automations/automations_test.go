package automations_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/automations"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/workspaces"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appautomations "github.com/kukv/hedgehug/internal/app/automations"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
)

const wsBase = "/api/stores/S1/workspaces"

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	_, api := humatest.New(t)
	me := apptest.Identity{Login: "taro"}
	workspaces.Register(api, appworkspaces.Service{Stores: s, IDs: ids, Identity: me, Inbox: &apptest.Inbox{}})
	automations.Register(api, appautomations.Service{Stores: s, IDs: ids, Identity: me, Clock: clock})
	return api
}

func idOf(t *testing.T, body []byte) string {
	t.Helper()
	var v struct{ ID string }
	if err := json.Unmarshal(body, &v); err != nil || v.ID == "" {
		t.Fatalf("body: %v %s", err, body)
	}
	return v.ID
}

func TestAutomationAPI(t *testing.T) {
	api := newAPI(t)
	res := api.Post(wsBase, map[string]string{"name": "Web", "prefix": "HH"})
	if res.Code != http.StatusCreated {
		t.Fatalf("workspace: %d %s", res.Code, res.Body)
	}
	ws := idOf(t, res.Body.Bytes())
	var d struct {
		Settings struct {
			Statuses []struct {
				ID string `json:"id"`
			} `json:"statuses"`
		} `json:"settings"`
	}
	_ = json.Unmarshal(api.Get(wsBase+"/"+ws).Body.Bytes(), &d)
	done := d.Settings.Statuses[len(d.Settings.Statuses)-1].ID
	path := wsBase + "/" + ws + "/automation"

	body := map[string]any{
		"name": "完了で外す", "trigger": "status", "triggerStatus": done,
		"allMerged": false, "action": "assignee", "value": "@none",
	}
	res = api.Post(path+"/rules", body)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	var created struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
		Name    string `json:"name"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &created)
	if !created.Enabled || created.Name != "完了で外す" {
		t.Errorf("created = %+v", created)
	}
	if res = api.Post(path+"/rules", body); res.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(res.Body.String(), "同じ名前のルールがあります") {
		t.Errorf("duplicate: %d %s", res.Code, res.Body)
	}
	var view struct {
		Rules  []map[string]any `json:"rules"`
		Recent []map[string]any `json:"recent"`
	}
	_ = json.Unmarshal(api.Get(path).Body.Bytes(), &view)
	if len(view.Rules) != 1 || view.Recent == nil {
		t.Errorf("view = %+v", view)
	}
	if res = api.Put(path+"/rules/"+created.ID, body); res.Code != http.StatusNoContent {
		t.Errorf("update: %d %s", res.Code, res.Body)
	}
	if res = api.Put(path+"/rules/"+created.ID+"/enabled", map[string]any{"enabled": false}); res.Code != http.StatusNoContent {
		t.Errorf("enabled: %d %s", res.Code, res.Body)
	}
	if res = api.Delete(path + "/rules/" + created.ID); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	bad := maps.Clone(body)
	bad["value"] = "jiro"
	if res = api.Post(path+"/rules", bad); res.Code != http.StatusNotFound {
		t.Errorf("unknown value: %d %s", res.Code, res.Body)
	}
	if res = api.Post("/api/stores/S1/items/nope/automation/nope/apply", map[string]any{"status": done, "from": done}); res.Code != http.StatusNotFound {
		t.Errorf("apply unknown item: %d %s", res.Code, res.Body)
	}
}
