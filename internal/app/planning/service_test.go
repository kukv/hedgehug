package planning

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

const store port.StoreID = "S1"

func TestWorksAndPlanOf(t *testing.T) {
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		ws, err := works(r, apptest.SampleWorkspace)
		if err != nil {
			return err
		}
		if len(ws) != 2 || len(ws[0].Log) == 0 {
			t.Errorf("works = %d 件 %+v", len(ws), ws)
		}
		if _, err := planOf(r, apptest.SampleWorkspace); err != nil {
			t.Errorf("planOf: %v", err)
		}
		if _, err := planOf(r, "01NOSUCHWS00000000000000"); err == nil {
			t.Error("planOf(unknown) = nil")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
