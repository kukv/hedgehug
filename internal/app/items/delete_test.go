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

// deleteTree は、サンプルの Epic HH-1 と Task HH-2 に、Story HH-3(Epic の下)、Step HH-4・HH-5(Task の下)を足す。
func deleteTree(t *testing.T, svc items.Service) (story item.Item, steps []item.Item) {
	t.Helper()
	story, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Story, Title: "請求"})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"下書き", "見直す"} {
		step, err := svc.AddStep(t.Context(), store, apptest.SampleTask, title)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, step)
	}
	return story, steps
}

// deletedRows は、アイテムのログの削除の行(delete・deleted)。
func deletedRows(t *testing.T, s port.Stores, id item.ID) []change.Record {
	t.Helper()
	var out []change.Record
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		for _, rec := range log {
			if rec.T == change.Delete && rec.F == change.DeletedField {
				out = append(out, rec)
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDeletePreviewCountsWhatGoesWithIt(t *testing.T) {
	svc, rec := setupRec(t)
	_, steps := deleteTree(t, svc)
	rec.SetRetentionDays(store, 7)
	cases := []struct {
		name  string
		id    item.ID
		count int
		kinds []item.Kind
	}{
		{"epic: story, task and steps", apptest.SampleEpic, 4, []item.Kind{item.Story, item.Task, item.Step}},
		{"task: steps", apptest.SampleTask, 2, []item.Kind{item.Step}},
	}
	for _, c := range cases {
		got, err := svc.DeletePreview(t.Context(), store, c.id)
		if err != nil || got.Count != c.count || !slices.Equal(got.Kinds, c.kinds) || got.RetentionDays != 7 {
			t.Errorf("%s: %+v, %v", c.name, got, err)
		}
	}
	if got, err := svc.DeletePreview(t.Context(), store, steps[0].ID); err != nil || got.Count != 0 || len(got.Kinds) != 0 {
		t.Errorf("a step: %+v, %v", got, err)
	}
}

func TestDeletePreviewLeavesOutWhatIsAlreadyDeleted(t *testing.T) {
	svc, _ := setupRec(t)
	_, steps := deleteTree(t, svc)
	if err := svc.Delete(t.Context(), store, steps[0].ID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.DeletePreview(t.Context(), store, apptest.SampleTask)
	if err != nil || got.Count != 1 || got.RetentionDays != 30 {
		t.Errorf("preview = %+v, %v", got, err)
	}
}

func TestDeleteWritesOneRowPerItemInOneOperation(t *testing.T) {
	svc, rec := setupRec(t)
	story, steps := deleteTree(t, svc)
	if err := svc.Delete(t.Context(), store, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-1 認証 ほか 4 件を削除" {
		t.Errorf("summary = %q", got)
	}
	all := []item.ID{apptest.SampleEpic, apptest.SampleTask, story.ID, steps[0].ID, steps[1].ID}
	op := deletedRows(t, rec, apptest.SampleEpic)[0].Op
	for _, id := range all {
		rows := deletedRows(t, rec, id)
		if len(rows) != 1 {
			t.Fatalf("%s: %d delete rows", id, len(rows))
		}
		r, root := rows[0], id == apptest.SampleEpic
		if r.From != nil || r.To != nil || r.Root == nil || *r.Root != root || r.Op != op {
			t.Errorf("%s: row %+v, want root %v in op %s", id, r, root, op)
		}
		if it := mustGet(t, rec, id); it.Deleted == nil || it.Deleted.Op != op || it.Deleted.Root != root {
			t.Errorf("%s: Deleted = %+v", id, it.Deleted)
		}
	}
}

// mustGet は、保存の口から直接アイテムを読む(削除済みも返る)。
func mustGet(t *testing.T, s port.Stores, id item.ID) item.Item {
	t.Helper()
	var it item.Item
	err := s.Read(t.Context(), store, func(r port.Reader) (err error) {
		it, err = r.Items().Get(id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func TestDeleteAStepTouchesNothingElse(t *testing.T) {
	svc, rec := setupRec(t)
	_, steps := deleteTree(t, svc)
	parent, sibling := logLen(t, rec, apptest.SampleTask), logLen(t, rec, steps[1].ID)
	if err := svc.Delete(t.Context(), store, steps[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-4 下書き を削除" {
		t.Errorf("summary = %q", got)
	}
	if logLen(t, rec, apptest.SampleTask) != parent || logLen(t, rec, steps[1].ID) != sibling {
		t.Error("the parent or a sibling got a row")
	}
}

// 別の操作で消したものには、あとから親を消しても行を足さない。復元はその削除の単位で行う(A4b-1)。
func TestDeleteKeepsAnEarlierDeletionAsItWas(t *testing.T) {
	svc, rec := setupRec(t)
	_, steps := deleteTree(t, svc)
	if err := svc.Delete(t.Context(), store, steps[0].ID); err != nil {
		t.Fatal(err)
	}
	before := deletedRows(t, rec, steps[0].ID)
	if err := svc.Delete(t.Context(), store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-2 ログイン画面を作る ほか 1 件を削除" {
		t.Errorf("summary = %q", got)
	}
	after := deletedRows(t, rec, steps[0].ID)
	if len(after) != 1 || after[0].ID != before[0].ID || after[0].Op != before[0].Op {
		t.Errorf("rows %+v → %+v", before, after)
	}
	if rows := deletedRows(t, rec, steps[1].ID); len(rows) != 1 || rows[0].Op == before[0].Op {
		t.Errorf("the other step: %+v", rows)
	}
}

func TestDeleteRefusesWhatIsGone(t *testing.T) {
	svc, rec := setupRec(t)
	if err := svc.Delete(t.Context(), store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	before := logLen(t, rec, apptest.SampleTask)
	if err := svc.Delete(t.Context(), store, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("twice: %v", err)
	}
	if _, err := svc.DeletePreview(t.Context(), store, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("preview of a deleted item: %v", err)
	}
	if err := svc.Delete(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	if _, err := svc.DeletePreview(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("preview of an unknown item: %v", err)
	}
	if logLen(t, rec, apptest.SampleTask) != before {
		t.Error("a refused delete wrote a row")
	}
}

// タイトルは改行を含みうる(同期で届いたもの)。アクティビティは 1 行でないと保存できないので、空白にまとめる。
func TestDeleteSummaryIsOneLine(t *testing.T) {
	svc, rec := setupRec(t)
	op := port.Operation{Actor: change.User("taro"), Summary: "タイトルを書き換え"}
	err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン\n  画面"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(t.Context(), store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-2 ログイン 画面 を削除" {
		t.Errorf("summary = %q", got)
	}
}
