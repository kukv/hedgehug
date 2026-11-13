package descriptions_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

var projectRef = change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}

// markDeleted は、アイテム・プロジェクトを削除済みにする(行を直接書く。root: true は利用者が選んだ削除)。
func markDeleted(t *testing.T, s port.Stores, ref change.Ref, root bool) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "削除"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		if ref.Kind == change.KindProject {
			return tx.WriteProjects().Append(ref.ID, change.NewDeleteEntity(root))
		}
		return tx.WriteItems().Append(ref.ID, change.NewDeleteEntity(root))
	}); err != nil {
		t.Fatal(err)
	}
}

// hasRestore は、ref のログに restore の行があるか。
func hasRestore(t *testing.T, s port.Stores, ref change.Ref) bool {
	t.Helper()
	return slices.Contains(logTypes(t, s, ref), change.Restore)
}

// 削除済みのアイテム・プロジェクトの説明は、読めない(A4b-4)。
func TestReadingDescriptionsOfDeletedOnesIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, taskRef, true)
	markDeleted(t, rec, projectRef, true)
	_, getItem := svc.GetItem(t.Context(), store, apptest.SampleTask)
	_, getProject := svc.GetProject(t.Context(), store, apptest.SampleProject)
	for name, err := range map[string]error{"GetItem": getItem, "GetProject": getProject} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
}

// 別のタブで削除された(root: true)Task の説明を保存すると、説明を残して削除を取り消す。
func TestSaveItemDescriptionDeletedRevives(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, taskRef, true)
	ctx := port.WithRevived(t.Context())
	got, err := svc.SaveItem(ctx, store, apptest.SampleTask, "", "本文")
	if err != nil || got.Text != "本文" {
		t.Fatalf("SaveItem = %+v, %v", got, err)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-2"}) {
		t.Errorf("Revived = %v", r)
	}
	if !hasRestore(t, rec, taskRef) {
		t.Error("restore の行がない")
	}
	if text, err := svc.GetItem(ctx, store, apptest.SampleTask); err != nil || text != "本文" {
		t.Errorf("GetItem = %q, %v", text, err)
	}
}

// 削除済み(root: true)のプロジェクトの説明を保存しても、同じ。
func TestSaveProjectDescriptionDeletedRevives(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, projectRef, true)
	ctx := port.WithRevived(t.Context())
	got, err := svc.SaveProject(ctx, store, apptest.SampleProject, "", "本文")
	if err != nil || got.Text != "本文" {
		t.Fatalf("SaveProject = %+v, %v", got, err)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-P1"}) {
		t.Errorf("Revived = %v", r)
	}
	if !hasRestore(t, rec, projectRef) {
		t.Error("restore の行がない")
	}
}

// 合わせた結果が保存済みと同じなら何も保存せず、削除済みのまま 404 になる(決定 S4)。
func TestSaveDescriptionDeletedUnchangedIsNotFound(t *testing.T) {
	for _, ref := range []change.Ref{taskRef, projectRef} {
		svc, rec := setup(t)
		markDeleted(t, rec, ref, true)
		ctx := port.WithRevived(t.Context())
		var err error
		if ref.Kind == change.KindProject {
			_, err = svc.SaveProject(ctx, store, apptest.SampleProject, "", "")
		} else {
			_, err = svc.SaveItem(ctx, store, apptest.SampleTask, "", "")
		}
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: Save = %v, want port.ErrNotFound", ref.Kind, err)
		}
		if hasRestore(t, rec, ref) {
			t.Errorf("%s: 同じ説明の保存で削除を取り消した", ref.Kind)
		}
	}
}

// 親と一緒に削除された(root: false)ものは、利用者が選んで削除していないので保存できず、削除済みのまま。
func TestSaveDescriptionDeletedWithParentIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	if err := apptest.DeleteItems(t.Context(), rec, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "本文"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SaveItem = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec, taskRef) {
		t.Error("断ったのに削除を取り消した")
	}
}

// プロジェクトも、root: false の削除済みなら保存できない。
func TestSaveProjectDescriptionDeletedWithParentIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, projectRef, false)
	if _, err := svc.SaveProject(t.Context(), store, apptest.SampleProject, "", "本文"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SaveProject = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec, projectRef) {
		t.Error("断ったのに削除を取り消した")
	}
}

// 削除済み(root: true)でも祖先(Epic)が削除済みなら、保存できず削除済みのまま。
func TestSaveItemDescriptionDeletedUnderDeletedAncestorIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	markDeleted(t, rec, change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic}, true)
	markDeleted(t, rec, taskRef, true)
	if _, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "本文"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SaveItem = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec, taskRef) {
		t.Error("断ったのに削除を取り消した")
	}
}
