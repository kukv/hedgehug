package bulkchange

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// failing は、Store fail の保存だけを失敗させる。
type failing struct {
	port.Stores
	fail port.StoreID
}

func (f failing) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if id == f.fail {
		return errors.New("boom")
	}
	return f.Stores.Write(ctx, id, op, fn)
}

// recorder は、保存を呼んだ Store とアクティビティの文を覚える。
type recorder struct {
	port.Stores
	summaries map[port.StoreID][]string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries[id] = append(r.summaries[id], op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

// setupRun は、setup の Store を、Store fail の保存を失敗させ、アクティビティの文を覚えるものに包む。
func setupRun(t *testing.T, fail port.StoreID) (Service, *apptest.Stores, *recorder) {
	t.Helper()
	svc, s := setup(t)
	rec := &recorder{Stores: failing{Stores: s, fail: fail}, summaries: map[port.StoreID][]string{}}
	svc.Stores = rec
	return svc, s, rec
}

func get(t *testing.T, s port.Stores, st port.StoreID, id item.ID) item.Item {
	t.Helper()
	var out item.Item
	err := s.Read(t.Context(), st, func(r port.Reader) (err error) {
		out, err = r.Items().Get(id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// addItems は、S1 に its を作る。
func addItems(t *testing.T, s port.Stores, its ...item.Item) {
	t.Helper()
	op := port.Operation{Actor: change.User("hanako"), Summary: "テストのアイテム"}
	err := s.Write(t.Context(), s1, op, func(tx port.Tx) error {
		for _, it := range its {
			if err := tx.WriteItems().Append(it.ID, item.Create(it)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// work は、サンプルのプロジェクトの、番号 n・種類 kind・Todo のアイテム。
func work(id item.ID, n int, kind item.Kind, rank string, sprint change.ID) item.Item {
	return item.Item{
		ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: n},
		Kind: kind, Rank: rank, Title: "作業", Status: apptest.SampleStatusTodo, Sprint: sprint,
	}
}

const (
	hh3     item.ID   = "01SAMPLEIT0000000000000013"
	hh4     item.ID   = "01SAMPLEIT0000000000000014"
	hh9     item.ID   = "01SAMPLEIT0000000000000019"
	sprint1 change.ID = "01SPRINT00000000000000001"
)

// addSprint は、S1 の Web に計画中のスプリント S-1 を作る。
func addSprint(t *testing.T, s port.Stores) {
	t.Helper()
	writeWs(t, s, s1, apptest.SampleWorkspace, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: sprint1, Name: "S-1", State: planning.Planned})...)
}

var done = Input{Field: FieldStatus, Value: "Done"}

func TestRunAcrossStores(t *testing.T) {
	svc, s, rec := setupRun(t, "")
	got, err := svc.Run(t.Context(), []Target{{s1, apptest.SampleEpic}, {s1, apptest.SampleTask}, {s2, apptest.SampleTask}, {s3, apptest.SampleTask}}, done)
	if err != nil {
		t.Fatal(err)
	}
	if got.Changed != 3 || !slices.Equal(got.Kept, []Kept{{s3, apptest.SampleTask, "閲覧のみ"}}) || len(got.Failed) != 0 {
		t.Errorf("Run = %+v", got)
	}
	if !slices.Equal(rec.summaries[s1], []string{"2 件のステータスを一括変更"}) || !slices.Equal(rec.summaries[s2], []string{"1 件のステータスを一括変更"}) {
		t.Errorf("summaries = %q", rec.summaries)
	}
	if _, ok := rec.summaries[s3]; ok {
		t.Errorf("the read-only store was written: %q", rec.summaries[s3])
	}
	for _, x := range []Target{{s1, apptest.SampleEpic}, {s1, apptest.SampleTask}, {s2, apptest.SampleTask}} {
		if it := get(t, s, x.Store, x.ID); it.Status != apptest.SampleStatusDone {
			t.Errorf("%v: status = %q", x, it.Status)
		}
	}
	if it := get(t, s, s3, apptest.SampleTask); it.Status != apptest.SampleStatusTodo {
		t.Errorf("read-only item changed: %q", it.Status)
	}
}

func TestRunSkipsSameAndCountsSelected(t *testing.T) {
	svc, s, rec := setupRun(t, "")
	writeItem(t, s, s1, apptest.SampleEpic, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	targets := []Target{{s1, apptest.SampleEpic}, {s1, apptest.SampleTask}}
	got, err := svc.Run(t.Context(), targets, done)
	if err != nil || got.Changed != 1 || len(got.Kept) != 0 {
		t.Fatalf("Run = %+v, %v", got, err)
	}
	if !slices.Equal(rec.summaries[s1], []string{"2 件のステータスを一括変更"}) {
		t.Errorf("summaries = %q", rec.summaries[s1])
	}
	got, err = svc.Run(t.Context(), targets, done)
	if err != nil || got.Changed != 0 || len(rec.summaries[s1]) != 1 {
		t.Errorf("all same: Run = %+v, %v, summaries = %q", got, err, rec.summaries[s1])
	}
}

// writeItem は、Store st のアイテム id のログに行 ds を書く。
func writeItem(t *testing.T, s port.Stores, st port.StoreID, id item.ID, ds ...change.Draft) {
	t.Helper()
	op := port.Operation{Actor: change.User("hanako"), Summary: "テストの行"}
	if err := s.Write(t.Context(), st, op, func(tx port.Tx) error { return tx.WriteItems().Append(id, ds...) }); err != nil {
		t.Fatal(err)
	}
}

func TestRunKeepsDeleted(t *testing.T) {
	svc, s, _ := setupRun(t, "")
	if err := apptest.DeleteItems(t.Context(), s, s1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Run(t.Context(), []Target{{s1, apptest.SampleEpic}, {s1, apptest.SampleTask}, {s1, "01SAMPLEIT0000000000000099"}}, done)
	if err != nil {
		t.Fatal(err)
	}
	want := []Kept{{s1, apptest.SampleTask, "削除済み"}, {s1, "01SAMPLEIT0000000000000099", "削除済み"}}
	if got.Changed != 1 || !slices.Equal(got.Kept, want) {
		t.Errorf("Run = %+v", got)
	}
	if it := get(t, s, s1, apptest.SampleTask); it.Deleted == nil || it.Status != apptest.SampleStatusTodo {
		t.Errorf("the deleted item was revived or changed: %+v", it)
	}
}

func TestBulkStoreFailure(t *testing.T) {
	svc, s, _ := setupRun(t, s2)
	targets := []Target{{s1, apptest.SampleEpic}, {s1, apptest.SampleTask}, {s2, apptest.SampleEpic}, {s2, apptest.SampleTask}}
	got, err := svc.Run(t.Context(), targets, done)
	if err != nil {
		t.Fatal(err)
	}
	if got.Changed != 2 || len(got.Kept) != 0 || !slices.Equal(got.Failed, []Failed{{s2, "team", 2}}) {
		t.Errorf("Run = %+v", got)
	}
	for _, x := range targets {
		want := apptest.SampleStatusDone
		if x.Store == s2 {
			want = apptest.SampleStatusTodo
		}
		if it := get(t, s, x.Store, x.ID); it.Status != want {
			t.Errorf("%v: status = %q, want %q", x, it.Status, want)
		}
	}
}

func TestBulkSprintKeepsOrder(t *testing.T) {
	svc, s, _ := setupRun(t, "")
	addSprint(t, s)
	addItems(t, s, work(hh9, 9, item.Story, "M", sprint1), work(hh3, 3, item.Story, "C", ""), work(hh4, 4, item.Story, "D", ""))
	got, err := svc.Run(t.Context(), []Target{{s1, hh4}, {s1, hh3}}, Input{Field: FieldSprint, Value: "S-1"})
	if err != nil || got.Changed != 2 {
		t.Fatalf("Run = %+v, %v", got, err)
	}
	its := []item.Item{get(t, s, s1, hh9), get(t, s, s1, hh3), get(t, s, s1, hh4)}
	for _, it := range its {
		if it.Sprint != sprint1 {
			t.Errorf("%s: sprint = %q", it.ID, it.Sprint)
		}
	}
	if its[0].Rank >= its[1].Rank || its[1].Rank >= its[2].Rank {
		t.Errorf("ranks = %q, %q, %q (want HH-9 < HH-3 < HH-4)", its[0].Rank, its[1].Rank, its[2].Rank)
	}
}

func TestBulkSprintEmptyFrameKeepsRanks(t *testing.T) {
	svc, s, _ := setupRun(t, "")
	addSprint(t, s)
	addItems(t, s, work(hh3, 3, item.Story, "C", ""))
	if _, err := svc.Run(t.Context(), []Target{{s1, hh3}}, Input{Field: FieldSprint, Value: "S-1"}); err != nil {
		t.Fatal(err)
	}
	if it := get(t, s, s1, hh3); it.Sprint != sprint1 || it.Rank != "C" {
		t.Errorf("item = %+v", it)
	}
}

func TestPreviewDoesNotWrite(t *testing.T) {
	svc, _, rec := setupRun(t, "")
	targets := []Target{{s1, apptest.SampleEpic}, {s1, apptest.SampleTask}, {s3, apptest.SampleTask}}
	pre, err := svc.Preview(t.Context(), targets, done)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.summaries) != 0 {
		t.Errorf("preview wrote: %q", rec.summaries)
	}
	if pre.ValueText != "Done" || pre.Changed != 2 || !slices.Equal(pre.Kept, []Kept{{s3, apptest.SampleTask, "閲覧のみ"}}) {
		t.Errorf("Preview = %+v", pre)
	}
	got, err := svc.Run(t.Context(), targets, done)
	if err != nil || got.Changed != pre.Changed || !slices.Equal(got.Kept, pre.Kept) || got.ValueText != pre.ValueText {
		t.Errorf("Run = %+v, %v; Preview = %+v", got, err, pre)
	}
	for _, c := range []struct {
		in   Input
		want string
	}{
		{Input{Field: FieldEstimate, Value: "1.5"}, "1.5 時間"},
		{Input{Field: FieldSprint}, "バックログ"},
		{Input{Field: FieldAssignee, Value: "HANAKO"}, "hanako"},
		{Input{Field: FieldPriority}, "なし"},
		{Input{Field: FieldPoints, Value: "3"}, "3"},
		{Input{Field: FieldSystemsAdd, Values: []string{" Web ", "API"}}, "Web, API"},
		{Input{Field: FieldStart, Value: "2026-11-20"}, "11/20"},
		{Input{Field: FieldStatus, Category: workspace.CategoryDone}, ""},
	} {
		got, err := svc.Preview(t.Context(), []Target{{s1, apptest.SampleTask}}, c.in)
		if err != nil || got.ValueText != c.want {
			t.Errorf("%+v: ValueText = %q, %v, want %q", c.in, got.ValueText, err, c.want)
		}
	}
}

func TestPreviewRejectsBadInput(t *testing.T) {
	svc, _, _ := setupRun(t, "")
	if _, err := svc.Preview(t.Context(), []Target{{s1, apptest.SampleTask}}, Input{Field: "title"}); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("err = %v", err)
	}
}

func TestRunUnknownStore(t *testing.T) {
	svc, s, rec := setupRun(t, "")
	_, err := svc.Run(t.Context(), []Target{{s1, apptest.SampleTask}, {"S9", apptest.SampleTask}}, done)
	if !errors.Is(err, port.ErrStoreNotFound) || len(rec.summaries) != 0 {
		t.Errorf("err = %v, summaries = %q", err, rec.summaries)
	}
	if it := get(t, s, s1, apptest.SampleTask); it.Status != apptest.SampleStatusTodo {
		t.Errorf("S1 was changed: %q", it.Status)
	}
}

func TestOffBacklog(t *testing.T) {
	for _, c := range []struct {
		value string
		want  OffBacklog
	}{
		{"Done", OffBacklog{Done: 1}},
		{"Cancel", OffBacklog{Canceled: 1}},
	} {
		svc, s, _ := setupRun(t, "")
		addSprint(t, s)
		addItems(t, s, work(hh3, 3, item.Story, "C", ""), work(hh4, 4, item.Story, "D", sprint1))
		got, err := svc.Run(t.Context(), []Target{{s1, hh3}, {s1, hh4}}, Input{Field: FieldStatus, Value: c.value})
		if err != nil || got.Changed != 2 || got.OffBacklog != c.want {
			t.Errorf("%s: Run = %+v, %v", c.value, got, err)
		}
	}
}
