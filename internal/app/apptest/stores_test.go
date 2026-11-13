package apptest_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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

// 同じ Store の Read どうしは互いを待たない(通しのテストのサーバーは、多くの画面の読み取りを同時に受ける)。
// Write は Read と重ならない。
func TestReadsOfOneStoreRunTogether(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	inner := make(chan error, 1)
	wrote := make(chan error, 1)
	err := s.Read(t.Context(), "S1", func(port.Reader) error {
		go func() { inner <- s.Read(t.Context(), "S1", func(port.Reader) error { return nil }) }()
		select {
		case err := <-inner:
			if err != nil {
				return err
			}
		case <-time.After(5 * time.Second):
			t.Error("a second Read waited for the first one")
		}
		go func() { wrote <- apptest.Sample(t.Context(), s, "S1") }()
		select {
		case err := <-wrote:
			t.Errorf("a Write ran while a Read was running (%v)", err)
			wrote <- err
		case <-time.After(50 * time.Millisecond):
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-wrote; err != nil {
		t.Fatal(err)
	}
}

// 本物(gitstore)の Read と Write は Store のファイルロック(LOCK_EX)を取るので、同じ goroutine の中で同じ Store を
// 入れ子に読み書きすると止まる。偽物は黙って通さず、panic で知らせる。別の Store なら入れ子でよい。
func TestNestedCallsOfOneStorePanic(t *testing.T) {
	ctx := t.Context()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	s.AddStore("S2")
	op := port.Operation{Actor: change.User("taro"), Summary: "入れ子"}
	read := func(id port.StoreID) error { return s.Read(ctx, id, func(port.Reader) error { return nil }) }
	write := func(id port.StoreID) error { return s.Write(ctx, id, op, func(port.Tx) error { return nil }) }
	inRead := func(inner func(port.StoreID) error) func() error {
		return func() error { return s.Read(ctx, "S1", func(port.Reader) error { return inner("S1") }) }
	}
	inWrite := func(inner func(port.StoreID) error) func() error {
		return func() error { return s.Write(ctx, "S1", op, func(port.Tx) error { return inner("S1") }) }
	}
	for name, call := range map[string]func() error{
		"Read_in_Read": inRead(read), "Write_in_Read": inRead(write),
		"Read_in_Write": inWrite(read), "Write_in_Write": inWrite(write),
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("did not panic")
				}
			}()
			_ = call()
		})
	}
	// panic のあとも、ロックを残さず読み書きできる。
	if err := s.Read(ctx, "S1", func(port.Reader) error { return read("S2") }); err != nil {
		t.Errorf("Read of another store in a Read = %v", err)
	}
	if err := write("S1"); err != nil {
		t.Errorf("Write after the panics = %v", err)
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

func TestFakeAddPurged(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	p := port.Purged{Entity: "item", Target: "01PURGEDIT0000000000000001", Display: "HH-12", At: time.Date(2026, 12, 12, 0, 0, 5, 0, time.UTC)}
	s.AddPurged("S1", p)
	err := s.Read(t.Context(), "S1", func(r port.Reader) error {
		got, err := r.Purged().List()
		if err != nil || len(got) != 1 || got[0] != p {
			t.Errorf("Purged = %+v, %v", got, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSampleMobileAndTeamDoNotShareInternalIDsWithTheSample(t *testing.T) {
	ctx := t.Context()
	s := apptest.NewStores(apptest.NewClock(time.Now()), apptest.NewIDs())
	s.AddStore("A")
	s.AddStore("B")
	if err := errors.Join(apptest.Sample(ctx, s, "A"), apptest.SampleMobile(ctx, s, "A"), apptest.SampleTeam(ctx, s, "B")); err != nil {
		t.Fatal(err)
	}
	if err := s.Read(ctx, "A", func(r port.Reader) error {
		ws, err := r.Workspaces().List()
		if err != nil || len(ws) != 2 {
			t.Errorf("A のワークスペース = %+v, %v", ws, err)
		}
		mobile, err := r.Workspaces().Get(apptest.SampleMobileWorkspace)
		if err != nil || mobile.Name != "Mobile" || mobile.Prefix != "MB" {
			t.Errorf("Mobile = %+v, %v", mobile, err)
		}
		epic, err := r.Items().Get(apptest.SampleMobileEpic)
		if err != nil || epic.Place.Number != 1 || epic.Place.Project != apptest.SampleMobileProject || epic.Title != "iOS" {
			t.Errorf("MB-1 = %+v, %v", epic, err)
		}
		st, err := r.Workspaces().Settings(apptest.SampleMobileWorkspace)
		if err != nil || st.Statuses[0].ID == apptest.SampleStatusTodo || epic.Status != st.Statuses[0].ID {
			t.Errorf("値の内部 ID が Web と同じか、Epic のステータスが Mobile の値でない: %+v, %v", st.Statuses, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	checkSampleTeam(t, s, "B")
}

// checkSampleTeam は、Store id の SampleTeam を確かめる。
func checkSampleTeam(t *testing.T, s port.Stores, id port.StoreID) {
	t.Helper()
	if err := s.Read(t.Context(), id, func(r port.Reader) error {
		ms, _ := r.Members().List()
		team, err := r.Workspaces().Get(apptest.SampleTeamWorkspace)
		epic, _ := r.Items().Get(apptest.SampleTeamEpic)
		if len(ms) != 2 || err != nil || team.Prefix != "TM" || epic.Place.Number != 1 || epic.Title != "チーム運営" {
			t.Errorf("Team: %+v %+v %+v, %v", ms, team, epic, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
