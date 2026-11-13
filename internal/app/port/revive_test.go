package port_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const reviveStore port.StoreID = "team"

// missingProject は、Store にないプロジェクトの内部 ID。
const missingProject project.ID = "01NOSUCHPR000000000000001"

var user = port.Operation{Actor: change.User("taro"), Summary: "削除"}

// reviveStores は、サンプル(Epic HH-1、その下の Task HH-2、プロジェクト HH-P1)を入れた偽物の Store。
func reviveStores(t *testing.T) *apptest.Stores {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore(reviveStore)
	if err := apptest.Sample(t.Context(), s, reviveStore); err != nil {
		t.Fatal(err)
	}
	return s
}

// deleteIn は、1 回の保存で、アイテム・プロジェクトに削除の行を書く(root の真偽を指定する)。
func deleteIn(t *testing.T, s port.Stores, items map[item.ID]bool, projects map[project.ID]bool) {
	t.Helper()
	err := s.Write(t.Context(), reviveStore, user, func(tx port.Tx) error {
		var errs []error
		for id, root := range items {
			errs = append(errs, tx.WriteItems().Append(id, change.NewDeleteEntity(root)))
		}
		for id, root := range projects {
			errs = append(errs, tx.WriteProjects().Append(id, change.NewDeleteEntity(root)))
		}
		return errors.Join(errs...)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// deleteTree は、Epic(root: true)と下の Task(root: false)を同じ操作で削除する。
func deleteTree(t *testing.T, s port.Stores) {
	t.Helper()
	deleteIn(t, s, map[item.ID]bool{apptest.SampleEpic: true, apptest.SampleTask: false}, nil)
}

func revive(t *testing.T, s port.Stores, fn func(ctx context.Context, tx port.Tx) error) []port.Revival {
	t.Helper()
	ctx := port.WithRevived(context.Background())
	if err := s.Write(ctx, reviveStore, user, func(tx port.Tx) error { return fn(ctx, tx) }); err != nil {
		t.Fatal(err)
	}
	return port.Revived(ctx)
}

func deletedOf(t *testing.T, s port.Stores, id item.ID) *change.Deletion {
	t.Helper()
	var d *change.Deletion
	err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		d = it.Deleted
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func restoreRows(t *testing.T, s port.Stores, ref change.Ref) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		log, err := r.Changes().Of(ref)
		for _, rec := range log {
			if rec.T == change.Restore {
				n++
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEditableItemReturnsRootDeleted(t *testing.T) {
	s := reviveStores(t)
	deleteTree(t, s)
	err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		it, err := port.EditableItem(r, apptest.SampleEpic)
		if err != nil || it.Deleted == nil {
			t.Errorf("epic = %+v, %v", it.Deleted, err)
		}
		if _, err := port.EditableItem(r, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("task: err = %v, want ErrNotFound", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditableProjectReturnsRootDeleted(t *testing.T) {
	s := reviveStores(t)
	deleteIn(t, s, nil, map[project.ID]bool{apptest.SampleProject: true})
	err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		if p, err := port.EditableProject(r, apptest.SampleProject); err != nil || p.Deleted == nil {
			t.Errorf("project = %+v, %v", p.Deleted, err)
		}
		if _, err := port.EditableProject(r, missingProject); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("missing: err = %v, want ErrNotFound", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s = reviveStores(t)
	deleteIn(t, s, nil, map[project.ID]bool{apptest.SampleProject: false})
	if err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		if _, err := port.EditableProject(r, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("root: false: err = %v, want ErrNotFound", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReviveItemRestoresTheWholeOp(t *testing.T) {
	s := reviveStores(t)
	deleteTree(t, s)
	got := revive(t, s, func(ctx context.Context, tx port.Tx) error {
		return port.ReviveItem(ctx, tx, apptest.SampleEpic)
	})
	if !slices.Equal(got, []port.Revival{{Kind: port.RevivedID, Text: "HH-1"}}) {
		t.Errorf("Revived = %v", got)
	}
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		if d := deletedOf(t, s, id); d != nil {
			t.Errorf("%s still deleted: %+v", id, d)
		}
	}
}

func TestReviveItemTwiceWritesOnce(t *testing.T) {
	s := reviveStores(t)
	deleteTree(t, s)
	got := revive(t, s, func(ctx context.Context, tx port.Tx) error {
		return errors.Join(port.ReviveItem(ctx, tx, apptest.SampleEpic), port.ReviveItem(ctx, tx, apptest.SampleEpic))
	})
	if !slices.Equal(got, []port.Revival{{Kind: port.RevivedID, Text: "HH-1"}}) {
		t.Errorf("Revived = %v", got)
	}
	if n := restoreRows(t, s, change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic}); n != 1 {
		t.Errorf("restore rows = %d, want 1", n)
	}
}

func TestReviveItemOnLiveDoesNothing(t *testing.T) {
	s := reviveStores(t)
	got := revive(t, s, func(ctx context.Context, tx port.Tx) error {
		return port.ReviveItem(ctx, tx, apptest.SampleEpic)
	})
	if len(got) != 0 {
		t.Errorf("Revived = %v", got)
	}
	if n := restoreRows(t, s, change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic}); n != 0 {
		t.Errorf("restore rows = %d, want 0", n)
	}
}

func TestReviveProjectRestoresItems(t *testing.T) {
	s := reviveStores(t)
	deleteIn(t, s,
		map[item.ID]bool{apptest.SampleEpic: false, apptest.SampleTask: false},
		map[project.ID]bool{apptest.SampleProject: true})
	got := revive(t, s, func(ctx context.Context, tx port.Tx) error {
		return port.ReviveProject(ctx, tx, apptest.SampleProject)
	})
	if !slices.Equal(got, []port.Revival{{Kind: port.RevivedID, Text: "HH-P1"}}) {
		t.Errorf("Revived = %v", got)
	}
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		if d := deletedOf(t, s, id); d != nil {
			t.Errorf("%s still deleted: %+v", id, d)
		}
	}
	err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		_, err := port.LiveProject(r, apptest.SampleProject)
		return err
	})
	if err != nil {
		t.Errorf("project still deleted: %v", err)
	}
}

func TestUnchanged(t *testing.T) {
	if err := port.Unchanged(nil); err != nil {
		t.Errorf("Unchanged(nil) = %v", err)
	}
	if err := port.Unchanged(&change.Deletion{}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Unchanged(&Deletion{}) = %v, want ErrNotFound", err)
	}
}

func TestReviveWithoutBoxStillRestores(t *testing.T) {
	s := reviveStores(t)
	deleteTree(t, s)
	if err := s.Write(t.Context(), reviveStore, user, func(tx port.Tx) error {
		return port.ReviveItem(t.Context(), tx, apptest.SampleEpic)
	}); err != nil {
		t.Fatal(err)
	}
	if got := port.Revived(t.Context()); got != nil {
		t.Errorf("Revived = %v, want nil", got)
	}
	if d := deletedOf(t, s, apptest.SampleEpic); d != nil {
		t.Errorf("epic still deleted: %+v", d)
	}
}

func TestEditableItemUnderParentDeletedByAnotherOp(t *testing.T) {
	s := reviveStores(t)
	deleteIn(t, s, map[item.ID]bool{apptest.SampleTask: true}, nil)
	deleteIn(t, s, map[item.ID]bool{apptest.SampleEpic: true}, nil)
	if err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		if _, err := port.EditableItem(r, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestEditableItemInProjectDeletedByAnotherOp(t *testing.T) {
	s := reviveStores(t)
	deleteIn(t, s, map[item.ID]bool{apptest.SampleTask: true}, nil)
	deleteIn(t, s, nil, map[project.ID]bool{apptest.SampleProject: true})
	if err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		if _, err := port.EditableItem(r, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReviveItemUnderDeletedParentDoesNothing(t *testing.T) {
	s := reviveStores(t)
	deleteIn(t, s, map[item.ID]bool{apptest.SampleTask: true}, nil)
	deleteIn(t, s, map[item.ID]bool{apptest.SampleEpic: true}, nil)
	got := revive(t, s, func(ctx context.Context, tx port.Tx) error {
		return port.ReviveItem(ctx, tx, apptest.SampleTask)
	})
	if len(got) != 0 || deletedOf(t, s, apptest.SampleTask) == nil {
		t.Errorf("Revived = %v, task deleted = %v", got, deletedOf(t, s, apptest.SampleTask) != nil)
	}
}

// 削除されていないアイテムは、祖先が削除済みでも読める(祖先を確かめるのは、削除済みのアイテムだけ)。
func TestEditableItemLiveItemUnderDeletedParent(t *testing.T) {
	s := reviveStores(t)
	deleteIn(t, s, map[item.ID]bool{apptest.SampleEpic: true}, nil)
	if err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		if it, err := port.EditableItem(r, apptest.SampleTask); err != nil || it.Deleted != nil {
			t.Errorf("task = %+v, %v", it.Deleted, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// addSameName は、サンプルのプロジェクト(HH-P1「ログイン」)と同じ名前のプロジェクト HH-P2 を作る。
func addSameName(t *testing.T, s port.Stores) project.ID {
	t.Helper()
	other := project.Project{ID: "01OTHERPR00000000000000001", Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "ログイン", Status: project.Todo}
	if err := s.Write(t.Context(), reviveStore, user, func(tx port.Tx) error {
		return tx.WriteProjects().Append(other.ID, project.Create(other))
	}); err != nil {
		t.Fatal(err)
	}
	return other.ID
}

// 同じ名前のプロジェクトが削除済みだけなら、ReviveProject は取り消せる(削除済みは名前を取らない)。
func TestReviveProjectWithSameNameDeletedProjectRevives(t *testing.T) {
	s := reviveStores(t)
	other := addSameName(t, s)
	deleteIn(t, s, nil, map[project.ID]bool{other: true})
	deleteIn(t, s, nil, map[project.ID]bool{apptest.SampleProject: true})
	got := revive(t, s, func(ctx context.Context, tx port.Tx) error {
		return port.ReviveProject(ctx, tx, apptest.SampleProject)
	})
	if !slices.Equal(got, []port.Revival{{Kind: port.RevivedID, Text: "HH-P1"}}) {
		t.Errorf("Revived = %v", got)
	}
}

// 同じ名前のプロジェクトがあっても、削除されていないプロジェクトへの ReviveProject は、何もせず断らない。
func TestReviveLiveProjectWithSameNameDoesNothing(t *testing.T) {
	s := reviveStores(t)
	addSameName(t, s)
	got := revive(t, s, func(ctx context.Context, tx port.Tx) error {
		return port.ReviveProject(ctx, tx, apptest.SampleProject)
	})
	if len(got) != 0 {
		t.Errorf("Revived = %v", got)
	}
	if n := restoreRows(t, s, change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}); n != 0 {
		t.Errorf("restore rows = %d, want 0", n)
	}
}

// 削除したプロジェクトと同じ名前の、削除されていないプロジェクトがあれば、ReviveProject は取り消さずに断る(決定 S11)。
func TestReviveProjectWithSameNameLiveProjectIsRefused(t *testing.T) {
	s := reviveStores(t)
	deleteIn(t, s, nil, map[project.ID]bool{apptest.SampleProject: true})
	addSameName(t, s)
	ctx := port.WithRevived(t.Context())
	err := s.Write(ctx, reviveStore, user, func(tx port.Tx) error { return port.ReviveProject(ctx, tx, apptest.SampleProject) })
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != "同じ名前のプロジェクトがあります。先にそのプロジェクトの名前を変更してください" {
		t.Fatalf("err = %v", err)
	}
	if n := restoreRows(t, s, change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}); n != 0 {
		t.Errorf("restore rows = %d, want 0", n)
	}
}

// review は、サンプルのワークスペースに足すステータス「Review」。
var review = workspace.Value{ID: "01REVIEW000000000000000001", Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}

var statuses = workspace.ParseList("statuses")

// writeAs は、利用者の 1 回の保存で fn を書く。
func writeAs(t *testing.T, s port.Stores, fn func(tx port.Tx) error) {
	t.Helper()
	if err := s.Write(t.Context(), reviveStore, user, fn); err != nil {
		t.Fatal(err)
	}
}

// deleteReview は、ステータス「Review」を足して Epic のステータスにしたあと、Epic のステータスを「Todo」に
// 置き換える行と同じ操作で「Review」を削除する。
func deleteReview(t *testing.T, s port.Stores) {
	t.Helper()
	writeAs(t, s, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, workspace.ValueDrafts(statuses, review)...),
			tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("status", apptest.SampleStatusTodo, review.ID)),
		)
	})
	writeAs(t, s, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete(statuses.Ref(review.ID))),
			tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("status", review.ID, apptest.SampleStatusTodo)),
		)
	})
}

// reviveSetting は、取り消しの入れ物を置いた 1 回の保存で、fs のそれぞれに ReviveSetting を呼び、
// 戻り値と積んだものを返す。
func reviveSetting(t *testing.T, s port.Stores, fs ...string) ([]bool, []port.Revival) {
	t.Helper()
	var ok []bool
	got := revive(t, s, func(ctx context.Context, tx port.Tx) error {
		for _, f := range fs {
			b, err := port.ReviveSetting(ctx, tx, apptest.SampleWorkspace, f)
			if err != nil {
				return err
			}
			ok = append(ok, b)
		}
		return nil
	})
	return ok, got
}

func settingsOf(t *testing.T, s port.Stores) workspace.Settings {
	t.Helper()
	var st workspace.Settings
	err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		var err error
		st, err = r.Workspaces().Settings(apptest.SampleWorkspace)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestReviveSettingRestoresTheWholeOp(t *testing.T) {
	s := reviveStores(t)
	deleteReview(t, s)
	ok, got := reviveSetting(t, s, statuses.Field(review.ID, "color"))
	if !slices.Equal(ok, []bool{true}) {
		t.Errorf("ReviveSetting = %v", ok)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "Review"}}; !slices.Equal(got, want) {
		t.Errorf("Revived = %v, want %v", got, want)
	}
	if !slices.ContainsFunc(settingsOf(t, s).Statuses, func(v workspace.Value) bool { return v.ID == review.ID }) {
		t.Error("Review is still deleted")
	}
	err := s.Read(t.Context(), reviveStore, func(r port.Reader) error {
		it, err := r.Items().Get(apptest.SampleEpic)
		if it.Status != review.ID {
			t.Errorf("epic status = %s, want Review", it.Status)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReviveSettingTwiceWritesOnce(t *testing.T) {
	s := reviveStores(t)
	deleteReview(t, s)
	f := statuses.Ref(review.ID)
	if ok, _ := reviveSetting(t, s, f, f); !slices.Equal(ok, []bool{true, false}) {
		t.Errorf("ReviveSetting = %v", ok)
	}
	if n := restoreRows(t, s, change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}); n != 1 {
		t.Errorf("restore rows = %d, want 1", n)
	}
}

func TestReviveSettingOnLiveDoesNothing(t *testing.T) {
	s := reviveStores(t)
	ok, got := reviveSetting(t, s, statuses.Ref(apptest.SampleStatusTodo))
	if !slices.Equal(ok, []bool{false}) || len(got) != 0 {
		t.Errorf("ReviveSetting = %v, Revived = %v", ok, got)
	}
	if n := restoreRows(t, s, change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}); n != 0 {
		t.Errorf("restore rows = %d, want 0", n)
	}
}

// テンプレートの子を先に、テンプレートをあとで、別々に削除する。子への変更は両方を取り消し、ログの順に積む。
func TestReviveSettingTemplateChild(t *testing.T) {
	s := reviveStores(t)
	const tid, cid change.ID = "01TEMPLATE0000000000000001", "01TEMPLATECHILD00000000001"
	tpl := workspace.Template{ID: tid, Name: "機能開発", Kind: "Epic", Children: []workspace.TemplateChild{{ID: cid, Kind: "Task", Title: "設計", Rank: "V"}}}
	writeAs(t, s, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, workspace.TemplateDrafts(workspace.Template{}, tpl)...)
	})
	child := workspace.TemplateField(tid, "children", string(cid))
	for _, f := range []string{child, workspace.TemplateField(tid)} {
		writeAs(t, s, func(tx port.Tx) error {
			return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete(f))
		})
	}
	ok, got := reviveSetting(t, s, child)
	if !slices.Equal(ok, []bool{true}) {
		t.Errorf("ReviveSetting = %v", ok)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "設計"}, {Kind: port.RevivedName, Text: "機能開発"}}; !slices.Equal(got, want) {
		t.Errorf("Revived = %v, want %v", got, want)
	}
	if back, found := settingsOf(t, s).Template(tid); !found || len(back.Children) != 1 {
		t.Errorf("template = %+v, %v", back, found)
	}
}

// 名前の改行と続いた空白は、1 つの空白にして積む。
func TestReviveSettingNameIsOneLine(t *testing.T) {
	s := reviveStores(t)
	v := workspace.Value{ID: "01MULTILINE000000000000001", Name: "Re\n  view", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	writeAs(t, s, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, workspace.ValueDrafts(statuses, v)...)
	})
	writeAs(t, s, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete(statuses.Ref(v.ID)))
	})
	_, got := reviveSetting(t, s, statuses.Ref(v.ID))
	if want := []port.Revival{{Kind: port.RevivedName, Text: "Re view"}}; !slices.Equal(got, want) {
		t.Errorf("Revived = %+v, want %+v", got, want)
	}
}
