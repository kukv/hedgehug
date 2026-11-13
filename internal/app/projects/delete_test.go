package projects_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

const (
	storyID  item.ID = "01DELETEIT0000000000000003"
	stepID   item.ID = "01DELETEIT0000000000000004"
	goneStep item.ID = "01DELETEIT0000000000000005"
)

// deleteFixture は、サンプルの Epic・Task に、Story(Epic の下)・Step(Task の下)・
// 先に別の操作で削除した Step を足す。
func deleteFixture(t *testing.T, s port.Stores) {
	t.Helper()
	mk := func(id, parent item.ID, number int, kind item.Kind, title string) item.Item {
		return item.Item{ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: number}, Kind: kind, Rank: "V", Title: title}
	}
	story := mk(storyID, apptest.SampleEpic, 3, item.Story, "請求")
	step := mk(stepID, apptest.SampleTask, 4, item.Step, "下書き")
	gone := mk(goneStep, apptest.SampleTask, 5, item.Step, "消した")
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(story.ID, item.Create(story)),
			tx.WriteItems().Append(step.ID, item.Create(step)),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	op.Summary = "先に削除"
	err = s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(gone.ID, item.Create(gone)),
			tx.WriteItems().Append(gone.ID, change.NewDeleteEntity(true)),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func deletedRow(t *testing.T, s port.Stores, ref change.Ref) *change.Record {
	t.Helper()
	var out *change.Record
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(ref)
		for _, rec := range log {
			if rec.T == change.Delete && rec.F == change.DeletedField {
				out = &rec
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDeletePreviewCountsLiveItems(t *testing.T) {
	svc, rec := setupEdit(t)
	deleteFixture(t, rec)
	rec.SetRetentionDays(store, 14)
	got, err := svc.DeletePreview(t.Context(), store, apptest.SampleProject)
	// Epic・Task・Story・Step の 4 件。先に削除した Step は数えない。
	if err != nil || got.Items != 4 || got.RetentionDays != 14 {
		t.Errorf("preview = %+v, %v", got, err)
	}
}

func TestDeleteWritesOneRowPerLiveItemInOneOperation(t *testing.T) {
	svc, rec := setupEdit(t)
	deleteFixture(t, rec)
	before := deletedRow(t, rec, change.Ref{Kind: change.KindItem, ID: goneStep})
	if err := svc.Delete(t.Context(), store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if got := rec.last(); got != "プロジェクト「ログイン」を削除" {
		t.Errorf("summary = %q", got)
	}
	pr := deletedRow(t, rec, change.Ref{Kind: change.KindProject, ID: apptest.SampleProject})
	if pr == nil || pr.Root == nil || !*pr.Root || pr.From != nil || pr.To != nil {
		t.Fatalf("project row = %+v", pr)
	}
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask, storyID, stepID} {
		row := deletedRow(t, rec, change.Ref{Kind: change.KindItem, ID: id})
		if row == nil || row.Op != pr.Op || row.Root == nil || *row.Root {
			t.Errorf("%s: row = %+v, want root false in op %s", id, row, pr.Op)
		}
	}
	after := deletedRow(t, rec, change.Ref{Kind: change.KindItem, ID: goneStep})
	if after.Op != before.Op || after.Op == pr.Op {
		t.Errorf("the step deleted earlier changed: %+v -> %+v", before, after)
	}
	err := rec.Read(t.Context(), store, func(r port.Reader) error {
		p, err := r.Projects().Get(apptest.SampleProject)
		if err != nil || p.Deleted == nil || p.Deleted.Op != pr.Op || !p.Deleted.Root {
			t.Errorf("Deleted = %+v, %v", p.Deleted, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// タイトルは改行を含みうる(作成と同じ)。アクティビティは 1 行でないと保存できないので、空白にまとめる。
func TestDeleteSummaryIsOneLine(t *testing.T) {
	svc, rec := setupEdit(t)
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject, change.NewSet("title", "ログイン", "決済\n  の流れ"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(t.Context(), store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if got := rec.last(); got != "プロジェクト「決済 の流れ」を削除" {
		t.Errorf("summary = %q", got)
	}
}

func TestDeleteRefusesWhatIsGone(t *testing.T) {
	svc, rec := setupEdit(t)
	if err := svc.Delete(t.Context(), store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	n := len(logOf(t, rec))
	if err := svc.Delete(t.Context(), store, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("twice: %v", err)
	}
	if _, err := svc.DeletePreview(t.Context(), store, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("preview of a deleted project: %v", err)
	}
	if err := svc.Delete(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	if _, err := svc.DeletePreview(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("preview of an unknown project: %v", err)
	}
	if len(logOf(t, rec)) != n {
		t.Error("a refused delete wrote a row")
	}
}
