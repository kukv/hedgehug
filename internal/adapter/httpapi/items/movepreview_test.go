package items_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

func TestMovePreview(t *testing.T) {
	api := newAPI(t, addSecondProject)
	epic := string(apptest.SampleEpic)
	res := api.Get("/api/stores/S1/items/" + epic + "/move-preview?project=" + secondProject)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"below":1`) || !strings.Contains(res.Body.String(), `"statuses":[]`) {
		t.Fatalf("preview: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + epic + "/move-preview?project=01NOPE00000000000000000000"); res.Code != http.StatusNotFound {
		t.Errorf("missing project: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + epic + "/move-preview"); res.Code != http.StatusNotFound {
		t.Errorf("no destination: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/stores/S1/items/"+epic+"/move", map[string]any{"project": secondProject, "statuses": map[string]string{"nope": "nope"}})
	if res.Code != http.StatusNotFound { // 置き換え先が移動先のステータスでないものは見つからない
		t.Errorf("move with unknown statuses: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/stores/S1/items/"+epic+"/move", map[string]any{"project": secondProject, "statuses": map[string]string{}})
	if res.Code != http.StatusNoContent {
		t.Errorf("move: %d %s", res.Code, res.Body)
	}
}
