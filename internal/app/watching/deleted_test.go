package watching_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

// 削除済みのアイテムはウォッチを変えられない(A4b-4)。
func TestWatchingADeletedItemIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(ctx, store, apptest.SampleTask, true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Set = %v, want port.ErrNotFound", err)
	}
}
