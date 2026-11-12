package stores_test

import (
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/stores"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
)

func TestListStores(t *testing.T) {
	_, api := humatest.New(t)
	stores.Register(api, appstores.Service{Stores: apptest.StoreList{Stores: []port.StoreInfo{
		{ID: "S1", Name: "e2e", Color: "sky"},
		{ID: "S2", Name: "readonly", Color: "orange", ReadOnly: true},
	}}})
	res := api.Get("/api/stores")
	want := `[{"id":"S1","name":"e2e","color":"sky","readOnly":false},{"id":"S2","name":"readonly","color":"orange","readOnly":true}]`
	if res.Code != http.StatusOK || res.Body.String() != want+"\n" {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
}

func TestListStoresEmptyIsAnArray(t *testing.T) {
	_, api := humatest.New(t)
	stores.Register(api, appstores.Service{Stores: apptest.StoreList{}})
	if res := api.Get("/api/stores"); res.Code != http.StatusOK || res.Body.String() != "[]\n" {
		t.Errorf("empty: %d %q", res.Code, res.Body)
	}
}
