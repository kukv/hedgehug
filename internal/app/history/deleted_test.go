package history_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

// 削除済みのアイテム・プロジェクトの履歴は読めない。削除の行は履歴に出ないので、削除する前の履歴も開かない(A4b-4)。
func TestHistoryOfDeletedOnesIsNotFound(t *testing.T) {
	svc, s := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteItems(ctx, s, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteProject(ctx, s, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Item(ctx, store, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Item = %v, want port.ErrNotFound", err)
	}
	if _, err := svc.Project(ctx, store, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Project = %v, want port.ErrNotFound", err)
	}
	if rows, err := svc.Item(ctx, store, apptest.SampleEpic); err != nil || len(rows) == 0 {
		t.Errorf("a live item's history = %+v, %v", rows, err)
	}
}
