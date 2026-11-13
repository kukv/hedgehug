package comments_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/comments"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// 削除済みのアイテム・プロジェクトのコメントは、読むことも書くこともできない(A4b-4)。
func TestCommentsOfADeletedOwnerAreNotFound(t *testing.T) {
	svc, rec := setup(t, "hanako")
	ctx := t.Context()
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteProject(ctx, rec.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	for _, target := range []comments.Target{
		{Kind: change.KindItem, ID: apptest.SampleTask},
		{Kind: change.KindProject, ID: apptest.SampleProject},
	} {
		_, listErr := svc.List(ctx, store, target)
		_, postErr := svc.Post(ctx, store, target, "確認")
		for name, err := range map[string]error{"List": listErr, "Post": postErr} {
			if !errors.Is(err, port.ErrNotFound) {
				t.Errorf("%s of a deleted %s = %v, want port.ErrNotFound", name, target.Kind, err)
			}
		}
	}
	// 削除していないアイテムには今までどおり書ける。
	if _, err := svc.Post(ctx, store, comments.Target{Kind: change.KindItem, ID: apptest.SampleEpic}, "確認"); err != nil {
		t.Errorf("Post to a live item = %v", err)
	}
}
