package notes_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/relations"
)

func newRelationsAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{Relations: relations.Service{Stores: s, Identity: apptest.Identity{Login: "taro"}}})
	return api
}

func TestRelationsRoundTrip(t *testing.T) {
	api := newRelationsAPI(t)
	epic, task := string(apptest.SampleEpic), string(apptest.SampleTask)
	if res := api.Get("/api/stores/S1/items/" + epic + "/relations"); res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf("empty: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/"+epic+"/relations", map[string]any{"label": "blocks", "other": task}); res.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", res.Code, res.Body)
	}
	res := api.Get("/api/stores/S1/items/" + epic + "/relations")
	body := res.Body.String()
	for _, want := range []string{`"label":"blocks"`, `"display":"HH-2"`, `"title":"ログイン画面を作る"`, `"id":"` + task + `"`, `"statusName":"Todo"`, `"workspaceName":""`} {
		if !strings.Contains(body, want) {
			t.Errorf("epic relations lack %s: %s", want, body)
		}
	}
	if res := api.Get("/api/stores/S1/items/" + task + "/relations"); !strings.Contains(res.Body.String(), `"label":"blocked by"`) || !strings.Contains(res.Body.String(), `"display":"HH-1"`) {
		t.Errorf("task relations: %s", res.Body)
	}
	res = api.Post("/api/stores/S1/items/"+epic+"/relations", map[string]any{"label": "blocks", "other": task})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "すでに blocks の関連があります") {
		t.Errorf("duplicate: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/"+task+"/relations/remove", map[string]any{"label": "blocked by", "other": epic}); res.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + epic + "/relations"); strings.TrimSpace(res.Body.String()) != "[]" {
		t.Errorf("after remove: %s", res.Body)
	}
}

func TestRelationCandidatesAndNotFound(t *testing.T) {
	api := newRelationsAPI(t)
	epic, task := string(apptest.SampleEpic), string(apptest.SampleTask)
	res := api.Get("/api/stores/S1/items/" + task + "/relation-candidates?label=blocks&q=hh")
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"display":"HH-1"`) || !strings.Contains(body, `"disabledReason":""`) || strings.Contains(body, `"display":"HH-2"`) {
		t.Errorf("candidates: %d %s", res.Code, body)
	}
	if res := api.Get("/api/stores/S1/items/" + task + "/relation-candidates?label=blocks&q="); strings.TrimSpace(res.Body.String()) != "[]" {
		t.Errorf("empty query: %s", res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + task + "/relation-candidates?label=friends&q=hh"); res.Code != http.StatusNotFound {
		t.Errorf("bad label: %d", res.Code)
	}
	if res := api.Post("/api/stores/S1/items/"+epic+"/relations", map[string]any{"label": "blocks", "other": "01NOPE00000000000000000000"}); res.Code != http.StatusNotFound {
		t.Errorf("no such item: %d", res.Code)
	}
}
