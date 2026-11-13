package items_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestCreateAcceptsADescriptionAndSavesIt(t *testing.T) {
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	items.Register(api, appitems.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}, Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))})

	res := api.Post("/api/stores/S1/items", map[string]any{
		"project": string(apptest.SampleProject), "parent": string(apptest.SampleEpic), "kind": "Task", "title": "API",
		"description": "## 手順\n",
	})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"number":3`) {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	var text string
	err := s.Read(t.Context(), "S1", func(r port.Reader) error {
		list, err := r.Items().List(apptest.SampleProject)
		if err != nil {
			return err
		}
		text, err = r.Items().Description(list[len(list)-1].ID)
		return err
	})
	if err != nil || text != "## 手順\n" {
		t.Errorf("description = %q, %v", text, err)
	}
}
