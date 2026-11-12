package stores_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/stores"
)

func TestAllKeepsTheSidebarOrder(t *testing.T) {
	want := []port.StoreInfo{
		{ID: "S2", Name: "work", Color: "sky"},
		{ID: "S1", Name: "oss", Color: "orange", ReadOnly: true},
	}
	got, err := stores.Service{Stores: apptest.StoreList{Stores: want}}.All(t.Context())
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("All = %+v, %v", got, err)
	}
}
