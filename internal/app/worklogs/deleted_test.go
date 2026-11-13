package worklogs_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

// 削除済みのアイテムの作業時間は、読むことも書くこともできない(A4b-4)。
func TestWorklogsOfADeletedItemAreNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	w, err := svc.Add(ctx, store, apptest.SampleTask, "1", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	_, listErr := svc.List(ctx, store, apptest.SampleTask)
	_, addErr := svc.Add(ctx, store, apptest.SampleTask, "1", "")
	deleteErr := svc.Delete(ctx, store, apptest.SampleTask, w.ID)
	for name, err := range map[string]error{"List": listErr, "Add": addErr, "Delete": deleteErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
}
