package worklogs_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// markDeleted は、アイテムを利用者が選んで削除した(root: true)ことにする(行を直接書く)。
func markDeleted(t *testing.T, s port.Stores, id change.ID) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "削除"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(id, change.NewDeleteEntity(true))
	}); err != nil {
		t.Fatal(err)
	}
}

// hasRestore は、Task(HH-2)のログに restore の行があるか。
func hasRestore(t *testing.T, s port.Stores) bool {
	t.Helper()
	found := false
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		found = slices.ContainsFunc(log, func(rec change.Record) bool { return rec.T == change.Restore })
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// 削除済みのアイテムの作業時間は、読めない(A4b-4)。
func TestListWorklogsOfADeletedItemIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, apptest.SampleTask)
	if _, err := svc.List(t.Context(), store, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List = %v, want port.ErrNotFound", err)
	}
}

// 別のタブで削除された(root: true)Task に作業時間を記録すると、記録を残して削除を取り消す。
func TestAddWorklogDeletedRevives(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, apptest.SampleTask)
	ctx := port.WithRevived(t.Context())
	v, err := svc.Add(ctx, store, apptest.SampleTask, "1.5", "戻して記録")
	if err != nil || v.Memo != "戻して記録" {
		t.Fatalf("Add = %+v, %v", v, err)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-2"}) {
		t.Errorf("Revived = %v", r)
	}
	if !hasRestore(t, rec) {
		t.Error("restore の行がない")
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 に 1.5 時間を記録" {
		t.Errorf("summary = %q", got)
	}
}

// 削除済み(root: true)の Task の作業時間の記録を削除すると、削除を取り消す。
func TestDeleteWorklogDeletedRevives(t *testing.T) {
	svc, rec := setup(t)
	w, err := svc.Add(t.Context(), store, apptest.SampleTask, "1", "")
	if err != nil {
		t.Fatal(err)
	}
	markDeleted(t, rec, apptest.SampleTask)
	ctx := port.WithRevived(t.Context())
	if err := svc.Delete(ctx, store, apptest.SampleTask, w.ID); err != nil {
		t.Fatal(err)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-2"}) {
		t.Errorf("Revived = %v", r)
	}
	if !hasRestore(t, rec) {
		t.Error("restore の行がない")
	}
	if list, err := svc.List(ctx, store, apptest.SampleTask); err != nil || len(list) != 0 {
		t.Errorf("List = %+v, %v", list, err)
	}
}

// 親と一緒に削除された(root: false)Task の作業時間は、追加も削除も断り、削除済みのまま。
func TestWorklogsOfAnItemDeletedWithParentAreNotFound(t *testing.T) {
	svc, rec := setup(t)
	w, err := svc.Add(t.Context(), store, apptest.SampleTask, "1", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(t.Context(), rec, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	_, addErr := svc.Add(t.Context(), store, apptest.SampleTask, "1", "")
	deleteErr := svc.Delete(t.Context(), store, apptest.SampleTask, w.ID)
	for name, err := range map[string]error{"Add": addErr, "Delete": deleteErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
	if hasRestore(t, rec) {
		t.Error("断ったのに削除を取り消した")
	}
}

// 削除済み(root: true)でも祖先の Epic が削除済みなら、記録を断り、削除済みのまま。
func TestAddWorklogDeletedUnderDeletedAncestorIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, apptest.SampleEpic)
	markDeleted(t, rec, apptest.SampleTask)
	if _, err := svc.Add(t.Context(), store, apptest.SampleTask, "1", ""); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec) {
		t.Error("断ったのに削除を取り消した")
	}
}
