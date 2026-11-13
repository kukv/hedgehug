package apptest_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// deletedSample は、サンプルの HH-2 とプロジェクトを削除済みにした Store。
func deletedSample(t *testing.T) (port.Stores, port.StoreID) {
	t.Helper()
	const store port.StoreID = "S1"
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(t.Context(), s, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteProject(t.Context(), s, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	return s, store
}

// 口は削除済みも返す(A4b-3)。
func TestReadersReturnDeletedOnes(t *testing.T) {
	s, store := deletedSample(t)
	mustRead(t, s, store, func(r port.Reader) error {
		if all, err := r.Items().List(apptest.SampleProject); err != nil || len(all) != 2 {
			t.Errorf("Items().List = %d items, %v", len(all), err)
		}
		if it, err := r.Items().Get(apptest.SampleTask); err != nil || it.Deleted == nil {
			t.Errorf("Items().Get(deleted) = %+v, %v", it.Deleted, err)
		}
		if all, err := r.Projects().List(apptest.SampleWorkspace); err != nil || len(all) != 1 {
			t.Errorf("Projects().List = %+v, %v", all, err)
		}
		return nil
	})
}

// Live… は削除済みを返さず、Get は ErrNotFound になる。削除されていないものとないものは今までどおり(A4b-4)。
func TestLiveReaders(t *testing.T) {
	s, store := deletedSample(t)
	mustRead(t, s, store, func(r port.Reader) error {
		if _, err := port.LiveItem(r, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("LiveItem(deleted) = %v", err)
		}
		if it, err := port.LiveItem(r, apptest.SampleEpic); err != nil || it.ID != apptest.SampleEpic {
			t.Errorf("LiveItem(live) = %+v, %v", it, err)
		}
		if _, err := port.LiveItem(r, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("LiveItem(unknown) = %v", err)
		}
		if its, err := port.LiveItems(r, apptest.SampleProject); err != nil || len(its) != 1 || its[0].ID != apptest.SampleEpic {
			t.Errorf("LiveItems = %+v, %v", its, err)
		}
		if _, err := port.LiveProject(r, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("LiveProject(deleted) = %v", err)
		}
		if ps, err := port.LiveProjects(r, apptest.SampleWorkspace); err != nil || len(ps) != 0 {
			t.Errorf("LiveProjects = %+v, %v", ps, err)
		}
		return nil
	})
}

// twoWorkspaces は、サンプルのワークスペースを削除済みにし、生きているワークスペース second を足した Store(B5b)。
func twoWorkspaces(t *testing.T, second workspace.ID) (port.Stores, port.StoreID) {
	t.Helper()
	const store port.StoreID = "S1"
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	d, err := workspace.Create(workspace.Workspace{ID: second, Name: "Second", Prefix: "SC"})
	if err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "ワークスペースを作成"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(second, d) }); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteWorkspace(t.Context(), s, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	return s, store
}

// ワークスペースの Live… は、削除済みを返さず、Get は ErrNotFound になる。口は削除済みも返す(B5b)。
func TestLiveWorkspace(t *testing.T) {
	const second workspace.ID = "01SECONDWS000000000000001"
	s, store := twoWorkspaces(t, second)
	mustRead(t, s, store, func(r port.Reader) error {
		if _, err := port.LiveWorkspace(r, apptest.SampleWorkspace); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("LiveWorkspace(deleted) = %v", err)
		}
		if _, err := port.LiveWorkspace(r, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("LiveWorkspace(unknown) = %v", err)
		}
		if w, err := port.LiveWorkspace(r, second); err != nil || w.ID != second {
			t.Errorf("LiveWorkspace(live) = %+v, %v", w, err)
		}
		return nil
	})
	mustRead(t, s, store, func(r port.Reader) error {
		if all, err := r.Workspaces().List(); err != nil || len(all) != 2 {
			t.Errorf("Workspaces().List must return the deleted one too: %+v, %v", all, err)
		}
		if ws, err := port.LiveWorkspaces(r); err != nil || len(ws) != 1 || ws[0].ID != second {
			t.Errorf("LiveWorkspaces = %+v, %v", ws, err)
		}
		return nil
	})
}

func mustRead(t *testing.T, s port.Stores, id port.StoreID, fn func(r port.Reader) error) {
	t.Helper()
	if err := s.Read(t.Context(), id, fn); err != nil {
		t.Fatal(err)
	}
}
