package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/views"
)

func TestViewWIP(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	save := func(method string, body map[string]any) views.SavedKanban {
		t.Helper()
		var out struct {
			View views.SavedView `json:"view"`
		}
		ts.call(method, "/api/views/b", body, 200, &out)
		return out.View.Kanban
	}
	if got := save("PUT", map[string]any{"layout": "kanban", "kanban_wip": map[string]int{"todo": 3, "doing": 2}}); len(got.WIP) != 2 || got.WIP["doing"] != 2 {
		t.Fatalf("saved = %+v", got)
	}
	// Other settings keep the limits; a new value replaces them all.
	if got := save("PATCH", map[string]any{"swimlane": "assignee"}); len(got.WIP) != 2 {
		t.Errorf("after a setting = %+v", got)
	}
	if got := save("PATCH", map[string]any{"kanban_wip": map[string]int{"doing": 4}}); len(got.WIP) != 1 || got.WIP["doing"] != 4 {
		t.Errorf("replaced = %+v", got)
	}
	if p := ts.problem("PATCH", "/api/views/b", map[string]any{"kanban_wip": map[string]int{"doing": 0}}, 400); p.Type != "view.bad_value" {
		t.Errorf("zero: %+v", p)
	}
	if got := save("PATCH", map[string]any{"kanban_wip": map[string]int{}}); got.WIP != nil {
		t.Errorf("taken off = %+v", got)
	}
}
