package apptest_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestFakeStores(t *testing.T) {
	apptest.RunStoresContract(t, func(t *testing.T) (port.Stores, port.StoreID, port.StoreID) {
		s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
		s.AddStore("S1")
		s.AddStore("S2")
		return s, "S1", "S2"
	})
}

func TestStoresVersions(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	s.AddStore("S2")
	var _ port.StoreVersions = s
	before, err := s.Versions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	after, err := s.Versions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if after["S1"] == before["S1"] {
		t.Errorf("S1 version did not change after a write: %q", after["S1"])
	}
	if after["S2"] != before["S2"] {
		t.Errorf("S2 version changed without a write: %q -> %q", before["S2"], after["S2"])
	}
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
	err = s.Read(t.Context(), "S1", func(r port.Reader) error {
		st, err := r.Workspaces().Settings(apptest.SampleWorkspace)
		if err != nil || st.InitialStatus != apptest.SampleStatusTodo || st.InitialPriority != apptest.SamplePriorityMedium {
			t.Errorf("settings = %+v, %v", st, err)
		}
		if it, err := r.Items().Get(apptest.SampleTask); err != nil || it.Status != apptest.SampleStatusTodo || it.Priority != apptest.SamplePriorityMedium {
			t.Errorf("sample task = %+v, %v", it, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// 同じ ID の Store を 2 度足すのはテストの書き間違いなので、黙って空の Store に替えずに panic する。
func TestAddStoreTwicePanics(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("AddStore of an existing id did not panic")
			}
		}()
		s.AddStore("S1")
	}()
	err := s.Read(t.Context(), "S1", func(r port.Reader) error {
		if ws, err := r.Workspaces().List(); err != nil || len(ws) != 1 {
			t.Errorf("the existing store was replaced: workspaces = %+v, %v", ws, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetRetentionDays(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	s.AddStore("S2")
	s.SetRetentionDays("S1", 7)
	for id, want := range map[port.StoreID]int{"S1": 7, "S2": 30} {
		err := s.Read(t.Context(), id, func(r port.Reader) error {
			if days, err := r.RetentionDays(); err != nil || days != want {
				t.Errorf("%s: RetentionDays = %d, %v; want %d", id, days, err, want)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
