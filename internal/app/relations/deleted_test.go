package relations_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// 相手が削除済みなら、関連は一覧・候補・追加・解除の相手から外れる(A4b-5)。
func TestARelationToADeletedItemIsHidden(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	addRaw(t, rec.Stores, apptest.SampleTask, "blocks", itDone) // HH-2 は HH-3 をブロックする
	if rows, err := svc.List(ctx, store, apptest.SampleTask); err != nil || len(rows) != 1 || rows[0].Item.ID != itDone {
		t.Fatalf("List before deleting = %+v, %v", rows, err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, itDone); err != nil {
		t.Fatal(err)
	}
	if rows, err := svc.List(ctx, store, apptest.SampleTask); err != nil || len(rows) != 0 {
		t.Errorf("List = %+v, %v; a deleted item must not be listed", rows, err)
	}
	if _, err := svc.List(ctx, store, itDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List of the deleted item = %v", err)
	}
	if cs, err := svc.Candidates(ctx, store, apptest.SampleTask, relation.RelatesTo, "決済"); err != nil || len(cs) != 0 {
		t.Errorf("Candidates = %+v, %v; a deleted item must not be a candidate", cs, err)
	}
	if err := svc.Add(ctx, store, apptest.SampleTask, relation.RelatesTo, itDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add to a deleted item = %v", err)
	}
	if err := svc.Remove(ctx, store, apptest.SampleTask, relation.Blocks, itDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Remove with a deleted item = %v", err)
	}
}

// 循環の検査は削除済みのアイテムをたどらない。関連の行そのものは消さない(復元で戻る。A4b-5)。
func TestTheCycleCheckSkipsDeletedItemsAndTheRowsStay(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	// HH-2 は HH-3 をブロックし、HH-3 は HH-1 をブロックする。HH-1 が HH-2 をブロックすると輪になる。
	addRaw(t, rec.Stores, apptest.SampleTask, "blocks", itDone)
	addRaw(t, rec.Stores, itDone, "blocks", apptest.SampleEpic)
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); message(err) != "循環になります" {
		t.Fatalf("before deleting: %v", err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, itDone); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Errorf("Add after deleting the link of the cycle: %v", err)
	}
	if rels := relationsOf(t, rec.Stores, apptest.SampleTask); len(rels) != 1 || rels[0] != (item.Relation{Type: "blocks", To: itDone}) {
		t.Errorf("relations of HH-2 = %+v; the row must stay", rels)
	}
}

// restored は、アイテムのログに restore の行があるか。
func restored(t *testing.T, s port.Stores, id item.ID) bool {
	t.Helper()
	var has bool
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		has = slices.ContainsFunc(log, func(r change.Record) bool { return r.T == change.Restore })
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return has
}

// taskDeleted は、Task(HH-2)が削除済みか。
func taskDeleted(t *testing.T, s port.Stores) bool {
	t.Helper()
	var d bool
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		it, err := r.Items().Get(apptest.SampleTask)
		d = it.Deleted != nil
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// wantTaskRevived は、Task(HH-2)の削除が取り消され、入れ物に HH-2 だけが積まれたことを確かめる。
func wantTaskRevived(ctx context.Context, t *testing.T, s port.Stores) {
	t.Helper()
	if taskDeleted(t, s) || !restored(t, s, apptest.SampleTask) {
		t.Error("削除を取り消していない")
	}
	if got := port.Revived(ctx); !slices.Equal(got, []string{"HH-2"}) {
		t.Errorf("Revived = %v", got)
	}
}

// 別のタブで削除された(root: true)アイテムから関連を足すと、関連を残して、そのアイテムだけ削除を取り消す。
// 「ブロックされている」は相手のログに書くが、取り消すのは操作したアイテム。
func TestAddRelationFromDeletedRevives(t *testing.T) {
	for _, tc := range []struct {
		label relation.Label
		owner item.ID
		rel   item.Relation
	}{
		{relation.Blocks, apptest.SampleTask, item.Relation{Type: "blocks", To: itDone}},
		{relation.BlockedBy, itDone, item.Relation{Type: "blocks", To: apptest.SampleTask}},
	} {
		t.Run(string(tc.label), func(t *testing.T) {
			svc, rec := setup(t)
			if err := apptest.DeleteItems(t.Context(), rec.Stores, store, apptest.SampleTask); err != nil {
				t.Fatal(err)
			}
			ctx := port.WithRevived(t.Context())
			if err := svc.Add(ctx, store, apptest.SampleTask, tc.label, itDone); err != nil {
				t.Fatal(err)
			}
			if rels := relationsOf(t, rec.Stores, tc.owner); !slices.Contains(rels, tc.rel) {
				t.Errorf("relations of the owner = %+v, want %+v", rels, tc.rel)
			}
			wantTaskRevived(ctx, t, rec.Stores)
			if restored(t, rec.Stores, itDone) {
				t.Error("相手のログに restore を書いた")
			}
		})
	}
}

// 相手が削除済みなら断り、自分(削除済み)は変わらない(決定 S5)。
func TestAddRelationToDeletedIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	if err := apptest.DeleteItems(t.Context(), rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(t.Context(), rec.Stores, store, itDone); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(t.Context(), store, apptest.SampleTask, relation.Blocks, itDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add = %v, want port.ErrNotFound", err)
	}
	if !taskDeleted(t, rec.Stores) || restored(t, rec.Stores, apptest.SampleTask) {
		t.Error("自分の削除を取り消した")
	}
}

// 親と一緒に削除された(root: false)アイテムからは、関連を足せず、外せない。
func TestRelationFromItemDeletedWithParentIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	addRaw(t, rec.Stores, apptest.SampleTask, "blocks", itDone)
	if err := apptest.DeleteItems(t.Context(), rec.Stores, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(t.Context(), store, apptest.SampleTask, relation.RelatesTo, itDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add = %v, want port.ErrNotFound", err)
	}
	if err := svc.Remove(t.Context(), store, apptest.SampleTask, relation.Blocks, itDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Remove = %v, want port.ErrNotFound", err)
	}
	if !taskDeleted(t, rec.Stores) || restored(t, rec.Stores, apptest.SampleTask) {
		t.Error("削除を取り消した")
	}
}

// 別のタブで削除されたアイテムの関連を外すと、外して削除を取り消す。外す関連がなければ断って削除済みのまま。
func TestRemoveRelationFromDeletedRevives(t *testing.T) {
	svc, rec := setup(t)
	addRaw(t, rec.Stores, apptest.SampleTask, "blocks", itDone)
	if err := apptest.DeleteItems(t.Context(), rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	ctx := port.WithRevived(t.Context())
	if err := svc.Remove(ctx, store, apptest.SampleTask, relation.Blocks, itDone); err != nil {
		t.Fatal(err)
	}
	if rels := relationsOf(t, rec.Stores, apptest.SampleTask); len(rels) != 0 {
		t.Errorf("relations = %+v", rels)
	}
	wantTaskRevived(ctx, t, rec.Stores)
	// 外す関連がもうない保存は、削除済みのままにして断る(決定 S4)
	if err := apptest.DeleteItems(t.Context(), rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	n := logLen(t, rec.Stores, apptest.SampleTask)
	if err := svc.Remove(t.Context(), store, apptest.SampleTask, relation.Blocks, itDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Remove with nothing to remove = %v, want port.ErrNotFound", err)
	}
	if !taskDeleted(t, rec.Stores) || logLen(t, rec.Stores, apptest.SampleTask) != n {
		t.Error("外す関連がないのに削除を取り消した")
	}
}

// 削除済みのプロジェクトの下の削除済みアイテムは、関連を足せず、削除済みのまま(決定 S9)。
func TestAddRelationFromItemInDeletedProjectIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	if err := apptest.DeleteItems(t.Context(), rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteProject(t.Context(), rec.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(t.Context(), store, apptest.SampleTask, relation.RelatesTo, itMobile); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add = %v, want port.ErrNotFound", err)
	}
	if !taskDeleted(t, rec.Stores) || restored(t, rec.Stores, apptest.SampleTask) {
		t.Error("削除を取り消した")
	}
}
