package items_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

func TestChangeItemKind(t *testing.T) {
	api := newAPI(t)
	url := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/kind"
	if res := api.Put(url, map[string]any{"kind": "Bug"}); res.Code != http.StatusNoContent {
		t.Fatalf("change: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + string(apptest.SampleTask)); !strings.Contains(res.Body.String(), `"kind":"Bug"`) {
		t.Errorf("changed: %s", res.Body)
	}
	for name, c := range map[string]struct {
		url, kind string
		code      int
	}{
		"同じ種類":         {url, "Bug", http.StatusNotFound},
		"Epic は変えられない": {"/api/stores/S1/items/" + string(apptest.SampleEpic) + "/kind", "Story", http.StatusNotFound},
		"Step にはできない":  {url, "Step", http.StatusUnprocessableEntity},
		"ないアイテム":       {"/api/stores/S1/items/01NOPE00000000000000000000/kind", "Bug", http.StatusNotFound},
	} {
		if res := api.Put(c.url, map[string]any{"kind": c.kind}); res.Code != c.code {
			t.Errorf("%s: %d %s", name, res.Code, res.Body)
		}
	}
}
