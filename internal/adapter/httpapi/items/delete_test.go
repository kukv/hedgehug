package items_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

func TestDeletePreviewAndDelete(t *testing.T) {
	api := newAPI(t)
	epic := "/api/stores/S1/items/" + string(apptest.SampleEpic)
	res := api.Get(epic + "/delete-preview")
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != `{"count":1,"kinds":["Task"],"retentionDays":30}` {
		t.Fatalf("preview: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/"+string(apptest.SampleTask)+"/steps", map[string]any{"title": "下書き"}); res.Code != http.StatusOK {
		t.Fatalf("add step: %d %s", res.Code, res.Body)
	}
	res = api.Get(epic + "/delete-preview")
	if !strings.Contains(res.Body.String(), `"count":2,"kinds":["Task","Step"]`) {
		t.Errorf("preview with a step: %s", res.Body)
	}
	if res := api.Delete(epic); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	if res := api.Get(epic + "/delete-preview"); res.Code != http.StatusNotFound {
		t.Errorf("preview after delete: %d", res.Code)
	}
	if res := api.Delete(epic); res.Code != http.StatusNotFound {
		t.Errorf("delete twice: %d", res.Code)
	}
	if res := api.Delete("/api/stores/S1/items/01NOPE00000000000000000000"); res.Code != http.StatusNotFound {
		t.Errorf("delete unknown: %d", res.Code)
	}
	if res := api.Get("/api/stores/S1/items/01NOPE00000000000000000000/delete-preview"); res.Code != http.StatusNotFound {
		t.Errorf("preview unknown: %d", res.Code)
	}
}

// 下のものがないときも kinds は null でなく []。
func TestDeletePreviewOfAnItemWithNothingBelow(t *testing.T) {
	api := newAPI(t)
	res := api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/delete-preview")
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != `{"count":0,"kinds":[],"retentionDays":30}` {
		t.Errorf("preview: %d %s", res.Code, res.Body)
	}
}
