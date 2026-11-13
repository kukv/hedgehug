package worklogs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/worklogs"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T) (worklogs.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return worklogs.Service{
		Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"},
		Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)),
	}, rec
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func logLen(t *testing.T, s port.Stores) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAddThenList(t *testing.T) {
	svc, rec := setup(t)
	v, err := svc.Add(t.Context(), store, apptest.SampleTask, " 1.25 ", " 下書き ")
	if err != nil {
		t.Fatal(err)
	}
	if v.Hundredths != 125 || v.Memo != "下書き" || v.Date != "2026-11-13" || v.By != "hanako" || v.ByName != "hanako" || v.ID == "" {
		t.Errorf("added %+v", v)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 に 1.25 時間を記録" {
		t.Errorf("summary = %q", got)
	}
	if _, err := svc.Add(t.Context(), store, apptest.SampleTask, ".5", ""); err != nil {
		t.Fatal(err)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 に 0.5 時間を記録" {
		t.Errorf("summary = %q", got)
	}
	list, err := svc.List(t.Context(), store, apptest.SampleTask)
	if err != nil || len(list) != 2 || list[0].Hundredths != 125 || list[1].Hundredths != 50 {
		t.Errorf("List = %+v, %v", list, err)
	}
}

func TestAddRefusesBadHoursAndSavesNothing(t *testing.T) {
	svc, rec := setup(t)
	before := logLen(t, rec)
	cases := map[string]string{
		"": "時間を入力してください", "0": "時間を入力してください", "-1": "時間を入力してください", "abc": "時間を入力してください",
		"0.123": "時間は小数第 2 位までで入力してください",
	}
	for in, want := range cases {
		if _, err := svc.Add(t.Context(), store, apptest.SampleTask, in, "x"); message(err) != want {
			t.Errorf("Add(%q) = %v, want %q", in, err, want)
		}
	}
	if logLen(t, rec) != before {
		t.Error("a refused record wrote a row")
	}
}

func TestAnyoneWhoCanWriteCanDeleteAnyRecord(t *testing.T) {
	svc, rec := setup(t)
	v, _ := svc.Add(t.Context(), store, apptest.SampleTask, "2", "")
	other := svc
	other.Identity = apptest.Identity{Login: "taro"}
	if err := other.Delete(t.Context(), store, apptest.SampleTask, v.ID); err != nil {
		t.Fatal(err)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 の作業時間を削除" {
		t.Errorf("summary = %q", got)
	}
	if list, _ := svc.List(t.Context(), store, apptest.SampleTask); len(list) != 0 {
		t.Errorf("List after delete = %+v", list)
	}
	if err := svc.Delete(t.Context(), store, apptest.SampleTask, v.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("deleting twice: %v", err)
	}
}

func TestAuthorMissingFromMembersIsAnUnknownUser(t *testing.T) {
	svc, _ := setup(t)
	svc.Identity = apptest.Identity{Login: "ghost"}
	v, err := svc.Add(t.Context(), store, apptest.SampleTask, "1", "")
	if err != nil || v.By != "ghost" || v.ByName != "不明なユーザー" {
		t.Errorf("added %+v, %v", v, err)
	}
}

func TestUnknownItem(t *testing.T) {
	svc, _ := setup(t)
	gone := change.ID("01NOPE00000000000000000000")
	if _, err := svc.List(t.Context(), store, gone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List: %v", err)
	}
	if _, err := svc.Add(t.Context(), store, gone, "1", ""); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add: %v", err)
	}
	if err := svc.Delete(t.Context(), store, apptest.SampleTask, gone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Delete of an unknown record: %v", err)
	}
}
