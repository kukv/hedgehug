package relations_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
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
