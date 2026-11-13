package items_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

func TestDuplicateItem(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/stores/S1/items/" + string(apptest.SampleTask) + "/duplicate")
	var out struct{ ID string }
	if res.Code != http.StatusOK || json.Unmarshal(res.Body.Bytes(), &out) != nil || out.ID == "" {
		t.Fatalf("duplicate: %d %s", res.Code, res.Body)
	}
	got := api.Get("/api/stores/S1/items/" + out.ID)
	if !strings.Contains(got.Body.String(), `"title":"ログイン画面を作る のコピー"`) || !strings.Contains(got.Body.String(), `"number":3`) {
		t.Errorf("duplicated: %s", got.Body)
	}
	if res := api.Post("/api/stores/S1/items/01NOPE00000000000000000000/duplicate"); res.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", res.Code)
	}
}
