package watching_test

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

// 別のタブで削除された(root: true)Task のウォッチを開始すると、ウォッチを残して削除を取り消す。
func TestWatchDeletedRevives(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, apptest.SampleTask)
	ctx := port.WithRevived(t.Context())
	if err := svc.Set(ctx, store, apptest.SampleTask, true); err != nil {
		t.Fatal(err)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-2"}) {
		t.Errorf("Revived = %v", r)
	}
	if !hasRestore(t, rec) {
		t.Error("restore の行がない")
	}
	if got := watchers(t, rec); !slices.Contains(got, "hanako") {
		t.Errorf("watchers = %v", got)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 のウォッチを開始" {
		t.Errorf("summary = %q", got)
	}
}

// すでにウォッチ中なら何も保存せず、削除済みのまま 404 になる(決定 S4)。
func TestWatchDeletedUnchangedIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil {
		t.Fatal(err)
	}
	markDeleted(t, rec, apptest.SampleTask)
	ctx := port.WithRevived(t.Context())
	if err := svc.Set(ctx, store, apptest.SampleTask, true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Set = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec) {
		t.Error("すでにウォッチ中の保存で削除を取り消した")
	}
}

// 親と一緒に削除された(root: false)Task、削除済み(root: true)でも祖先の Epic が削除済みの Task は、
// ウォッチを変えられず、削除済みのまま。
func TestWatchDeletedWithParentOrUnderDeletedAncestorIsNotFound(t *testing.T) {
	t.Run("root: false", func(t *testing.T) {
		svc, rec := setup(t)
		if err := apptest.DeleteItems(t.Context(), rec, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
			t.Fatal(err)
		}
		if err := svc.Set(t.Context(), store, apptest.SampleTask, true); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Set = %v, want port.ErrNotFound", err)
		}
		if hasRestore(t, rec) {
			t.Error("断ったのに削除を取り消した")
		}
	})
	t.Run("祖先が削除済み", func(t *testing.T) {
		svc, rec := setup(t)
		markDeleted(t, rec, apptest.SampleEpic)
		markDeleted(t, rec, apptest.SampleTask)
		if err := svc.Set(t.Context(), store, apptest.SampleTask, true); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Set = %v, want port.ErrNotFound", err)
		}
		if hasRestore(t, rec) {
			t.Error("断ったのに削除を取り消した")
		}
	})
}
