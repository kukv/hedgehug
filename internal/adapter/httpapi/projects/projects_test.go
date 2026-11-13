package projects_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
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
	projects.Register(api, appprojects.Service{
		Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"},
		Clock: apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)),
	})
	return api
}

const base = "/api/stores/S1/workspaces/" + string(apptest.SampleWorkspace) + "/projects"

func TestCreateAndList(t *testing.T) {
	api := newAPI(t)
	res := api.Post(base, map[string]any{"title": "決済", "owner": "taro", "status": "InProgress", "start": "2026-11-01", "end": ""})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"number":2`) || !strings.Contains(res.Body.String(), `"status":"InProgress"`) {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	res = api.Get(base)
	if res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 2 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"title":"ログイン"`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
}

func TestRefusalIs422WithTheMessage(t *testing.T) {
	res := newAPI(t).Post(base, map[string]any{"title": "ログイン"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "このワークスペースには同じ名前のプロジェクトがあります") {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}

func TestBadInputIsRefusedByTheSchema(t *testing.T) {
	api := newAPI(t)
	for _, body := range []map[string]any{
		{"title": "x", "status": "Review"}, {"title": "x", "start": "11/01"}, {"title": "x", "end": "2026-02-30"}, {"title": "a\nb"},
	} {
		if res := api.Post(base, body); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%v: %d", body, res.Code)
		}
	}
}

func TestUnknownStoreIs404(t *testing.T) {
	if res := newAPI(t).Get("/api/stores/nope/projects/x"); res.Code != http.StatusNotFound {
		t.Errorf("%d", res.Code)
	}
}
