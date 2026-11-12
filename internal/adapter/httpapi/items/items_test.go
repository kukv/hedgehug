package items_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appitems "github.com/kukv/hedgehug/internal/app/items"
)

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	items.Register(api, appitems.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}, Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))})
	return api
}

func TestCreateGetList(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/stores/S1/items", map[string]any{
		"project": string(apptest.SampleProject), "parent": string(apptest.SampleEpic), "kind": "Task", "title": "API",
	})
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"number":3`) || !strings.Contains(body, `"watchers":["taro"]`) || !strings.Contains(body, `"createdBy":"taro"`) {
		t.Fatalf("create: %d %s", res.Code, body)
	}
	if res := api.Get("/api/stores/S1/items/" + string(apptest.SampleEpic)); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"parent":null`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject) + "/items"); res.Code != http.StatusOK || strings.Count(res.Body.String(), `"kind":`) != 3 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
}

func TestRefusals(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/stores/S1/items", map[string]any{"project": string(apptest.SampleProject), "kind": "Task", "title": "画面"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "親になる Epic を選んでください") {
		t.Errorf("no parent: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/stores/S1/items", map[string]any{"project": "", "kind": "Task", "title": "画面"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "プロジェクトを選んでください") {
		t.Errorf("no project: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items", map[string]any{"project": "x", "kind": "Saga", "title": "x"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad kind: %d", res.Code)
	}
	res = api.Post("/api/stores/S1/items", map[string]any{"project": "01OTHERPR00000000000000001", "parent": string(apptest.SampleEpic), "kind": "Task", "title": "画面"})
	if res.Code != http.StatusUnprocessableEntity {
		t.Errorf("parent elsewhere: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/01NOPE00000000000000000000"); res.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", res.Code)
	}
}
