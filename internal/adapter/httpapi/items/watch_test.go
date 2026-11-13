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
	appwatching "github.com/kukv/hedgehug/internal/app/watching"
)

func newWatchAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	me := apptest.Identity{Login: "hanako"}
	_, api := humatest.New(t)
	svc := appitems.Service{Stores: s, IDs: ids, Identity: me, Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))}
	items.RegisterWatch(api, svc, appwatching.Service{Stores: s, Identity: me})
	return api
}

func TestSetWatch(t *testing.T) {
	api := newWatchAPI(t)
	path := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/watch"
	res := api.Put(path, map[string]any{"on": true})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"watchers":["hanako","taro"]`) {
		t.Errorf("start: %d %s", res.Code, res.Body)
	}
	res = api.Put(path, map[string]any{"on": false})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"watchers":["taro"]`) {
		t.Errorf("stop: %d %s", res.Code, res.Body)
	}
	if res := api.Put(path, map[string]any{"on": false}); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"watchers":["taro"]`) {
		t.Errorf("stop again: %d %s", res.Code, res.Body)
	}
	if res := api.Put("/api/stores/S1/items/01NOPE00000000000000000000/watch", map[string]any{"on": true}); res.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", res.Code)
	}
}
