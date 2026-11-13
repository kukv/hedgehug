package items_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

func TestRenameItem(t *testing.T) {
	api := newAPI(t)
	res := api.Put("/api/stores/S1/items/"+string(apptest.SampleTask)+"/title", map[string]any{"title": " 画面を\n直す "})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"title":"画面を 直す"`) {
		t.Errorf("rename: %d %s", res.Code, res.Body)
	}
	res = api.Put("/api/stores/S1/items/"+string(apptest.SampleTask)+"/title", map[string]any{"title": " "})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "タイトルは空にできません") {
		t.Errorf("empty: %d %s", res.Code, res.Body)
	}
	if res := api.Put("/api/stores/S1/items/01NOPE00000000000000000000/title", map[string]any{"title": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", res.Code)
	}
}

func TestAddStep(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/stores/S1/items/"+string(apptest.SampleTask)+"/steps", map[string]any{"title": "下書き"})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"kind":"Step"`) || !strings.Contains(res.Body.String(), `"number":3`) {
		t.Fatalf("add step: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/stores/S1/items/"+string(apptest.SampleEpic)+"/steps", map[string]any{"title": "下書き"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "Step は Epic の下に置けません") {
		t.Errorf("under epic: %d %s", res.Code, res.Body)
	}
}

func TestReorder(t *testing.T) {
	api := newAPI(t)
	task := "/api/stores/S1/items/" + string(apptest.SampleTask)
	var steps []string
	for _, title := range []string{"a", "b"} {
		res := api.Post(task+"/steps", map[string]any{"title": title})
		if res.Code != http.StatusOK {
			t.Fatalf("add step: %d %s", res.Code, res.Body)
		}
		var body struct{ ID, Rank string }
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		steps = append(steps, body.ID)
	}
	one := "/api/stores/S1/items/" + steps[1] + "/reorder"
	if res := api.Post(one, map[string]any{"anchor": steps[0], "after": false}); res.Code != http.StatusNoContent {
		t.Errorf("reorder: %d %s", res.Code, res.Body)
	}
	res := api.Get("/api/stores/S1/items/" + steps[1])
	var moved, first struct{ Rank string }
	_ = json.Unmarshal(res.Body.Bytes(), &moved)
	_ = json.Unmarshal(api.Get("/api/stores/S1/items/"+steps[0]).Body.Bytes(), &first)
	if moved.Rank == "" || moved.Rank >= first.Rank {
		t.Errorf("b %q must come before a %q", moved.Rank, first.Rank)
	}
	if res := api.Post(one, map[string]any{"anchor": string(apptest.SampleEpic), "after": true}); res.Code != http.StatusNotFound {
		t.Errorf("anchor of another parent: %d", res.Code)
	}
	if res := api.Post("/api/stores/S1/items/01NOPE00000000000000000000/reorder", map[string]any{"anchor": steps[0], "after": true}); res.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", res.Code)
	}
	if res := api.Post(one, map[string]any{"after": true}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("no anchor: %d", res.Code)
	}
	if res := api.Post("/api/stores/S1/items/" + steps[1] + "/move-up"); res.Code != http.StatusNotFound && res.Code != http.StatusMethodNotAllowed {
		t.Errorf("move-up must be gone: %d", res.Code)
	}
}

func TestNumbers(t *testing.T) {
	api := newAPI(t)
	ws := "/api/stores/S1/workspaces/" + string(apptest.SampleWorkspace)
	if res := api.Get(ws + "/next-item-number"); res.Code != http.StatusOK || res.Body.String() != `{"number":3}`+"\n" {
		t.Errorf("next: %d %s", res.Code, res.Body)
	}
	if res := api.Get(ws + "/items/by-number/2"); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), string(apptest.SampleTask)) {
		t.Errorf("by number: %d %s", res.Code, res.Body)
	}
	if res := api.Get(ws + "/items/by-number/9"); res.Code != http.StatusNotFound {
		t.Errorf("by number 9: %d", res.Code)
	}
	if res := api.Get("/api/stores/S1/workspaces/01NOPE00000000000000000000/next-item-number"); res.Code != http.StatusNotFound {
		t.Errorf("next unknown ws: %d", res.Code)
	}
}
