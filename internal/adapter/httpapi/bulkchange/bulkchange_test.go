package bulkchange_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/bulkchange"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appbulkchange "github.com/kukv/hedgehug/internal/app/bulkchange"
	"github.com/kukv/hedgehug/internal/app/port"
)

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	bulkchange.Register(api, appbulkchange.Service{
		Stores: s, Identity: apptest.Identity{Login: "taro"}, Clock: clock,
		StoreList: apptest.StoreList{Stores: []port.StoreInfo{{ID: "S1", Name: "S1"}}},
	})
	return api
}

func targets(store string, ids ...string) []map[string]string {
	out := []map[string]string{}
	for _, id := range ids {
		out = append(out, map[string]string{"store": store, "item": id})
	}
	return out
}

func body(extra map[string]any, ids ...string) map[string]any {
	b := map[string]any{"items": targets("S1", ids...)}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

var both = []string{string(apptest.SampleEpic), string(apptest.SampleTask)}

func TestBulkChangeAPI(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/bulk-change", body(map[string]any{"field": "status", "value": "Done"}, both...))
	s := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(s, `"changed":2`) || !strings.Contains(s, `"kept":[]`) || !strings.Contains(s, `"failed":[]`) {
		t.Errorf("run: %d %s", res.Code, s)
	}
}

func TestBulkChangePreviewAPI(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/bulk-change/preview", body(map[string]any{"field": "status", "value": "Done"}, both...))
	s := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(s, `"changed":2`) || !strings.Contains(s, `"valueText":"Done"`) || !strings.Contains(s, `"failed":[]`) || !strings.Contains(s, `"kept":[]`) {
		t.Errorf("preview: %d %s", res.Code, s)
	}
	res = api.Post("/api/bulk-change/preview", body(map[string]any{"field": "points", "value": "-1"}, both...))
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "ポイントは 0 以上の数で入力してください") {
		t.Errorf("points: %d %s", res.Code, res.Body)
	}
}

func TestBulkChangeChoicesAPI(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/bulk-change/choices", body(nil, both...))
	s := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(s, `"statuses":["Todo"`) || !strings.Contains(s, `{"login":"taro","name":"taro"}`) {
		t.Errorf("choices: %d %s", res.Code, s)
	}
}

func TestBulkChangeAPIRejects(t *testing.T) {
	api := newAPI(t)
	for _, path := range []string{"/api/bulk-change", "/api/bulk-change/preview", "/api/bulk-change/choices"} {
		extra := map[string]any{"field": "status", "value": "Done"}
		if path == "/api/bulk-change/choices" {
			extra = nil // 選択肢の本文は items だけ
		}
		if res := api.Post(path, body(extra)); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s empty: %d %s", path, res.Code, res.Body)
		}
		unknown := body(extra)
		unknown["items"] = targets("NOPE", both[0])
		if res := api.Post(path, unknown); res.Code != http.StatusNotFound {
			t.Errorf("%s unknown store: %d %s", path, res.Code, res.Body)
		}
	}
	for _, path := range []string{"/api/bulk-change", "/api/bulk-change/preview"} {
		if res := api.Post(path, body(map[string]any{"field": "title", "value": "x"}, both...)); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s field: %d %s", path, res.Code, res.Body)
		}
		if res := api.Post(path, body(map[string]any{"field": "start", "value": "あした"}, both...)); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s bad date: %d %s", path, res.Code, res.Body)
		}
	}
}
