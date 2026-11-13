package workspaces_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/workspaces"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appprojectrules "github.com/kukv/hedgehug/internal/app/projectrules"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
)

func TestDetailHasProjectRules(t *testing.T) {
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	workspaces.Register(api, appworkspaces.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}})
	url := base + "/" + string(apptest.SampleWorkspace)
	if res := api.Get(url); !strings.Contains(res.Body.String(), `"projectRules":{}`) {
		t.Fatalf("empty: %s", res.Body)
	}
	pr := appprojectrules.Service{Stores: s, Identity: apptest.Identity{Login: "taro"}}
	if err := pr.SetStatusUsed(t.Context(), "S1", apptest.SampleProject, apptest.SampleStatusPending, false); err != nil {
		t.Fatal(err)
	}
	res := api.Get(url)
	want := `"projectRules":{"` + string(apptest.SampleProject) + `":{"hiddenStatuses":["` + string(apptest.SampleStatusPending) + `"],"workflow":{},"templates":[],"templateDefaults":{},"disabledRules":[]}}`
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), want) {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
}
