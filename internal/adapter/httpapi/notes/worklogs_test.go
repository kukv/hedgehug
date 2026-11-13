package notes_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appworklogs "github.com/kukv/hedgehug/internal/app/worklogs"
)

func newWorklogsAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{
		Worklogs: appworklogs.Service{
			Stores: s, IDs: ids, Identity: apptest.Identity{Login: "hanako"},
			Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)),
		},
	})
	return api
}

func TestWorklogLifecycle(t *testing.T) {
	api := newWorklogsAPI(t)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/worklogs"
	res := api.Post(base, map[string]any{"hours": "1.25", "memo": "下書き"})
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"hours":"1.25"`) || !strings.Contains(body, `"date":"2026-11-13"`) ||
		!strings.Contains(body, `"memo":"下書き"`) || !strings.Contains(body, `"byName":"hanako"`) {
		t.Fatalf("add: %d %s", res.Code, body)
	}
	var added notes.Worklog
	if err := json.Unmarshal(res.Body.Bytes(), &added); err != nil || added.ID == "" {
		t.Fatalf("added = %+v, %v", added, err)
	}
	if res := api.Get(base); res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 1 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	if res := api.Delete(base + "/" + added.ID); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	if res := api.Get(base); res.Code != http.StatusOK || res.Body.String() != "[]\n" {
		t.Errorf("list after delete: %d %q", res.Code, res.Body)
	}
	if res := api.Delete(base + "/" + added.ID); res.Code != http.StatusNotFound {
		t.Errorf("delete twice: %d", res.Code)
	}
}

func TestWorklogRefusalsAndUnknowns(t *testing.T) {
	api := newWorklogsAPI(t)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/worklogs"
	res := api.Post(base, map[string]any{"hours": "0.123"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "時間は小数第 2 位までで入力してください") {
		t.Errorf("scale: %d %s", res.Code, res.Body)
	}
	res = api.Post(base, map[string]any{"hours": ""})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "時間を入力してください") {
		t.Errorf("empty: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/01NOPE00000000000000000000/worklogs", map[string]any{"hours": "1"}); res.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", res.Code)
	}
}
