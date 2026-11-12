package apptest_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestFakeStores(t *testing.T) {
	apptest.RunStoresContract(t, func(t *testing.T) (port.Stores, port.StoreID) {
		s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
		s.AddStore("S1")
		return s, "S1"
	})
}

func TestSample(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	err := s.Read(t.Context(), "S1", func(r port.Reader) error {
		ws, err := r.Workspaces().List()
		if err != nil || len(ws) != 1 || ws[0].Prefix != "HH" {
			t.Errorf("workspaces = %+v, %v", ws, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
