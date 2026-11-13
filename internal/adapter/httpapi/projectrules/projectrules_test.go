package projectrules_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/projectrules"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appprojectrules "github.com/kukv/hedgehug/internal/app/projectrules"
)

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	projectrules.Register(api, appprojectrules.Service{Stores: s, Identity: apptest.Identity{Login: "taro"}})
	return api
}

const rules = "/api/stores/S1/projects/" + string(apptest.SampleProject) + "/rules"

func TestRoutesSave(t *testing.T) {
	api := newAPI(t)
	for _, c := range []struct {
		name string
		code func() int
	}{
		{"status", func() int {
			return api.Put(rules+"/statuses/"+string(apptest.SampleStatusPending), map[string]any{"used": false}).Code
		}},
		{"workflow", func() int {
			return api.Put(rules+"/workflow/"+string(apptest.SampleStatusTodo)+"/"+string(apptest.SampleStatusDone), map[string]any{"allowed": true}).Code
		}},
		{"default none", func() int { return api.Put(rules+"/template-defaults/Task", map[string]any{"template": "@none"}).Code }},
		{"reset", func() int { return api.Post(rules + "/statuses/reset").Code }},
	} {
		if got := c.code(); got != http.StatusNoContent {
			t.Errorf("%s: %d", c.name, got)
		}
	}
}

func TestRefusalIs422WithTheMessage(t *testing.T) {
	api := newAPI(t)
	// 初期ステータスは外せない
	res := api.Put(rules+"/statuses/"+string(apptest.SampleStatusTodo), map[string]any{"used": false})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "外せません") {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
}

func TestMissingIs404(t *testing.T) {
	api := newAPI(t)
	gone := "/api/stores/S1/projects/01NOPE00000000000000000000/rules"
	if res := api.Put(gone+"/statuses/"+string(apptest.SampleStatusPending), map[string]any{"used": false}); res.Code != http.StatusNotFound {
		t.Errorf("project: %d %s", res.Code, res.Body)
	}
	if res := api.Post(rules + "/nope/reset"); res.Code != http.StatusNotFound {
		t.Errorf("section: %d %s", res.Code, res.Body)
	}
	if res := api.Put(rules+"/automation/01NOPE", map[string]any{"runs": false}); res.Code != http.StatusNotFound {
		t.Errorf("rule: %d %s", res.Code, res.Body)
	}
}
