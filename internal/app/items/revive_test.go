package items_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// hasRestore は、Task(HH-2)のログに restore の行があるか。
func hasRestore(t *testing.T, s port.Stores) bool {
	t.Helper()
	return slices.ContainsFunc(records(t, s, apptest.SampleTask), func(r change.Record) bool { return r.T == change.Restore })
}

// 別のタブで削除された(root: true)Task のタイトルを保存すると、変更を残して削除を取り消す。
func TestRenameDeletedItemRevives(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	got, err := svc.Rename(ctx, store, apptest.SampleTask, "新しい")
	if err != nil {
		t.Fatal(err)
	}
	if got.Deleted != nil || got.Title != "新しい" {
		t.Errorf("Rename = %+v", got)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []port.Revival{{Kind: port.RevivedID, Text: "HH-2"}}) {
		t.Errorf("Revived = %v", r)
	}
	if n := len(slices.DeleteFunc(slices.Clone(rec.summaries), func(s string) bool { return s != "HH-2 のタイトルを変更" })); n != 1 {
		t.Errorf("summary の保存 = %d 件", n)
	}
	if !hasRestore(t, rec) {
		t.Error("restore の行がない")
	}
}

// タイトルが同じなら何も保存せず、削除済みのまま 404 になる(決定 S4)。
func TestRenameDeletedItemSameTitleIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	before := storedItem(t, rec, apptest.SampleTask)
	ctx := port.WithRevived(t.Context())
	if _, err := svc.Rename(ctx, store, apptest.SampleTask, before.Title); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Rename = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec) || storedItem(t, rec, apptest.SampleTask).Deleted == nil || len(port.Revived(ctx)) != 0 {
		t.Error("同じタイトルの保存で削除を取り消した")
	}
}

// 親と一緒に削除された(root: false)ものは、利用者が選んで削除していないので保存できない。
func TestRenameItemDeletedWithParentIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	if err := apptest.DeleteItems(t.Context(), rec, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rename(t.Context(), store, apptest.SampleTask, "新しい"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Rename = %v, want port.ErrNotFound", err)
	}
}

// Epic の削除を取り消すと、同じ操作で一緒に削除された下の Task も戻る。
func TestRenameDeletedEpicRevivesChildren(t *testing.T) {
	svc, rec := setupRec(t)
	if err := apptest.DeleteItems(t.Context(), rec, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	ctx := port.WithRevived(t.Context())
	if _, err := svc.Rename(ctx, store, apptest.SampleEpic, "新しい"); err != nil {
		t.Fatal(err)
	}
	if storedItem(t, rec, apptest.SampleTask).Deleted != nil || storedItem(t, rec, apptest.SampleEpic).Deleted != nil {
		t.Error("Epic と下の Task が戻っていない")
	}
}

func TestChangeKindDeletedItemRevives(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	if err := svc.ChangeKind(ctx, store, apptest.SampleTask, item.Bug); err != nil {
		t.Fatal(err)
	}
	if got := storedItem(t, rec, apptest.SampleTask); got.Deleted != nil || got.Kind != item.Bug {
		t.Errorf("item = %+v", got)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []port.Revival{{Kind: port.RevivedID, Text: "HH-2"}}) {
		t.Errorf("Revived = %v", r)
	}
}

func TestReorderDeletedItemRevives(t *testing.T) {
	svc, rec := setupRec(t)
	other := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Task, "兄弟")
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	if err := svc.Reorder(ctx, store, apptest.SampleTask, other.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := storedItem(t, rec, apptest.SampleTask); got.Deleted != nil || got.Rank <= other.Rank {
		t.Errorf("item = %+v (兄弟 %q のあとに戻る)", got, other.Rank)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []port.Revival{{Kind: port.RevivedID, Text: "HH-2"}}) {
		t.Errorf("Revived = %v", r)
	}
}

func TestMoveDeletedItemRevives(t *testing.T) {
	svc, rec := setupRec(t)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	if err := svc.Move(ctx, store, apptest.SampleTask, items.MoveInput{Parent: &epic2.ID}); err != nil {
		t.Fatal(err)
	}
	got := storedItem(t, rec, apptest.SampleTask)
	if got.Deleted != nil || got.Place.Parent == nil || *got.Place.Parent != epic2.ID {
		t.Errorf("item = %+v", got)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []port.Revival{{Kind: port.RevivedID, Text: "HH-2"}}) {
		t.Errorf("Revived = %v", r)
	}
}

// 移動先の親が削除済みなら 404。移すアイテムは削除済みのまま変わらない。
func TestMoveToDeletedParentIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	markDeleted(t, rec, epic2.ID, true)
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	if err := svc.Move(ctx, store, apptest.SampleTask, items.MoveInput{Parent: &epic2.ID}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Move = %v, want port.ErrNotFound", err)
	}
	if got := storedItem(t, rec, apptest.SampleTask); got.Deleted == nil || hasRestore(t, rec) || len(port.Revived(ctx)) != 0 {
		t.Errorf("移せなかったのに取り消した: %+v", got)
	}
}

// Task を削除したあと、親の Epic も別の保存で削除された。Task は祖先が削除済みなので保存できない(決定 S9)。
func TestRenameItemUnderParentDeletedLaterIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	markDeleted(t, rec, apptest.SampleEpic, true)
	if _, err := svc.Rename(t.Context(), store, apptest.SampleTask, "新しい"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Rename = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec) {
		t.Error("祖先が削除済みなのに restore を書いた")
	}
}

// 削除済み(root: true)でも、断られる保存は 404 のまま、削除を取り消さない。
func TestChangeKindToTheSameKindOfDeletedItemIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	if err := svc.ChangeKind(ctx, store, apptest.SampleTask, item.Task); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ChangeKind = %v, want port.ErrNotFound", err)
	}
	if storedItem(t, rec, apptest.SampleTask).Deleted == nil || hasRestore(t, rec) {
		t.Error("断られたのに削除を取り消した")
	}
}

// 何も変わらない移動は、削除済み(root: true)なら 404 で、削除を取り消さない。
func TestMoveDeletedItemToTheSameParentIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	if err := svc.Move(ctx, store, apptest.SampleTask, items.MoveInput{Parent: ptr(apptest.SampleEpic)}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Move = %v, want port.ErrNotFound", err)
	}
	if storedItem(t, rec, apptest.SampleTask).Deleted == nil || hasRestore(t, rec) || len(port.Revived(ctx)) != 0 {
		t.Error("何も変わらない移動で削除を取り消した")
	}
}

// すでにその位置への並べ替えは、削除済み(root: true)なら 404 で、削除を取り消さない。
func TestReorderDeletedItemAlreadyInPlaceIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	other := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Task, "兄弟")
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	// HH-2 は兄弟の前にいるので、兄弟の直前はすでにその位置。
	if err := svc.Reorder(ctx, store, apptest.SampleTask, other.ID, false); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Reorder = %v, want port.ErrNotFound", err)
	}
	if storedItem(t, rec, apptest.SampleTask).Deleted == nil || hasRestore(t, rec) {
		t.Error("すでにその位置なのに削除を取り消した")
	}
}
