package items_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

func bulkBody(store, id string) map[string]any {
	return map[string]any{"items": []map[string]string{{"store": store, "item": id}}}
}

// 一括削除の対象に、Step を 2 件持つ Task を使う。
func bulkAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	api := newAPI(t)
	for _, title := range []string{"下書き", "清書"} {
		if res := api.Post("/api/stores/S1/items/"+string(apptest.SampleTask)+"/steps", map[string]any{"title": title}); res.Code != http.StatusOK {
			t.Fatalf("add step: %d %s", res.Code, res.Body)
		}
	}
	return api
}

func TestBulkDeletePreviewAPI(t *testing.T) {
	api := bulkAPI(t)
	res := api.Post("/api/bulk-delete/preview", bulkBody("S1", string(apptest.SampleTask)))
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != `{"stores":1,"retentionDays":30,"readOnly":0}` {
		t.Errorf("preview: %d %s", res.Code, res.Body)
	}
}

func TestBulkDeleteAPI(t *testing.T) {
	api := bulkAPI(t)
	res := api.Post("/api/bulk-delete", bulkBody("S1", string(apptest.SampleTask)))
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != `{"deleted":3,"readOnly":0}` {
		t.Fatalf("delete: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + string(apptest.SampleTask)); res.Code != http.StatusNotFound {
		t.Errorf("get after delete: %d", res.Code)
	}
}

func TestBulkDeleteAPIRejects(t *testing.T) {
	api := bulkAPI(t)
	for _, path := range []string{"/api/bulk-delete", "/api/bulk-delete/preview"} {
		if res := api.Post(path, map[string]any{"items": []map[string]string{}}); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s empty: %d %s", path, res.Code, res.Body)
		}
		if res := api.Post(path, map[string]any{"items": nil}); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s null: %d %s", path, res.Code, res.Body)
		}
		if res := api.Post(path, bulkBody("NOPE", string(apptest.SampleTask))); res.Code != http.StatusNotFound {
			t.Errorf("%s unknown store: %d %s", path, res.Code, res.Body)
		}
	}
}
