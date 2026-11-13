package descriptions_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

// 削除済みのアイテム・プロジェクトの説明は、読むことも保存することもできない(A4b-4)。
func TestDescriptionsOfDeletedOnesAreNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteProject(ctx, rec.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	_, getItem := svc.GetItem(ctx, store, apptest.SampleTask)
	_, saveItem := svc.SaveItem(ctx, store, apptest.SampleTask, "", "本文")
	_, getProject := svc.GetProject(ctx, store, apptest.SampleProject)
	_, saveProject := svc.SaveProject(ctx, store, apptest.SampleProject, "", "本文")
	for name, err := range map[string]error{"GetItem": getItem, "SaveItem": saveItem, "GetProject": getProject, "SaveProject": saveProject} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
}
