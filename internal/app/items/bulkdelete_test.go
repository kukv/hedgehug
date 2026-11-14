package items_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

const (
	store2 port.StoreID = "S2"
	store3 port.StoreID = "S3"
)

// setupBulk は、S1・S2(サンプル)と S3(サンプル、閲覧のみ)の 3 つの Store を持つ Service を返す。
func setupBulk(t *testing.T) (items.Service, *recorder, *apptest.Stores) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	for _, id := range []port.StoreID{store, store2, store3} {
		s.AddStore(id)
		if err := apptest.Sample(t.Context(), s, id); err != nil {
			t.Fatal(err)
		}
	}
	rec := &recorder{Stores: s}
	list := &apptest.StoreList{Stores: []port.StoreInfo{{ID: store, Name: "main"}, {ID: store2, Name: "team"}, {ID: store3, Name: "readonly", ReadOnly: true}}}
	svc := items.Service{Stores: rec, StoreList: list, IDs: ids, Identity: apptest.Identity{Login: "hanako"}, Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))}
	return svc, rec, s
}

// rowsIn は、Store st のアイテムのログの削除の行。
func rowsIn(t *testing.T, s port.Stores, st port.StoreID, id item.ID) []change.Record {
	t.Helper()
	var out []change.Record
	err := s.Read(t.Context(), st, func(r port.Reader) error {
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

func TestBulkDeleteAcrossStores(t *testing.T) {
	svc, rec, s := setupBulk(t)
	_, steps := deleteTree(t, svc)
	before := len(rec.summaries)
	got, err := svc.BulkDelete(t.Context(), []items.BulkTarget{
		{Store: store, ID: apptest.SampleTask}, {Store: store2, ID: apptest.SampleEpic}, {Store: store3, ID: apptest.SampleTask},
	})
	if err != nil || got != (items.BulkDeleted{Deleted: 5, ReadOnly: 1}) {
		t.Fatalf("BulkDelete = %+v, %v", got, err)
	}
	if want := []string{"3 件を一括削除", "2 件を一括削除"}; !slices.Equal(rec.summaries[before:], want) {
		t.Errorf("activities = %q, want %q", rec.summaries[before:], want)
	}
	if rows := rowsIn(t, s, store, apptest.SampleTask); len(rows) != 1 || rows[0].Root == nil || !*rows[0].Root {
		t.Errorf("S1 task: %+v", rows)
	}
	for _, st := range steps {
		if rows := rowsIn(t, s, store, st.ID); len(rows) != 1 || rows[0].Root == nil || *rows[0].Root {
			t.Errorf("S1 %s: %+v", st.ID, rows)
		}
	}
	if rows := rowsIn(t, s, store3, apptest.SampleTask); len(rows) != 0 {
		t.Errorf("the read-only store got rows: %+v", rows)
	}
}

func TestBulkDeleteOverlap(t *testing.T) {
	svc, rec, s := setupBulk(t)
	story, steps := deleteTree(t, svc)
	before := len(rec.summaries)
	got, err := svc.BulkDelete(t.Context(), []items.BulkTarget{{Store: store, ID: story.ID}, {Store: store, ID: apptest.SampleEpic}})
	if err != nil || got != (items.BulkDeleted{Deleted: 5}) {
		t.Fatalf("BulkDelete = %+v, %v", got, err)
	}
	if want := []string{"5 件を一括削除"}; !slices.Equal(rec.summaries[before:], want) {
		t.Errorf("activities = %q", rec.summaries[before:])
	}
	for _, c := range []struct {
		id   item.ID
		root bool
	}{{apptest.SampleEpic, true}, {story.ID, false}, {apptest.SampleTask, false}, {steps[0].ID, false}, {steps[1].ID, false}} {
		if rows := rowsIn(t, s, store, c.id); len(rows) != 1 || rows[0].Root == nil || *rows[0].Root != c.root {
			t.Errorf("%s: rows %+v, want one with root %v", c.id, rows, c.root)
		}
	}
}

func TestBulkDeleteSkipsDeleted(t *testing.T) {
	svc, rec, _ := setupBulk(t)
	story, _ := deleteTree(t, svc)
	if err := svc.Delete(t.Context(), store, story.ID); err != nil {
		t.Fatal(err)
	}
	targets := []items.BulkTarget{{Store: store, ID: story.ID}, {Store: store, ID: apptest.SampleTask}}
	got, err := svc.BulkDelete(t.Context(), targets)
	if err != nil || got != (items.BulkDeleted{Deleted: 3}) {
		t.Fatalf("BulkDelete = %+v, %v", got, err)
	}
	before := len(rec.summaries)
	got, err = svc.BulkDelete(t.Context(), targets)
	if err != nil || got != (items.BulkDeleted{}) || len(rec.summaries) != before {
		t.Errorf("all deleted: %+v, %v, %d activities added", got, err, len(rec.summaries)-before)
	}
}

func TestBulkDeleteUnknownStore(t *testing.T) {
	svc, rec, s := setupBulk(t)
	before := len(rec.summaries)
	_, err := svc.BulkDelete(t.Context(), []items.BulkTarget{{Store: store, ID: apptest.SampleTask}, {Store: "S9", ID: apptest.SampleTask}})
	if !errors.Is(err, port.ErrStoreNotFound) {
		t.Fatalf("err = %v", err)
	}
	if len(rec.summaries) != before || len(rowsIn(t, s, store, apptest.SampleTask)) != 0 {
		t.Error("something was written")
	}
	if _, err := svc.BulkDeletePreview(t.Context(), []items.BulkTarget{{Store: "S9", ID: apptest.SampleTask}}); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("preview err = %v", err)
	}
}

func TestBulkDeletePreview(t *testing.T) {
	svc, _, s := setupBulk(t)
	s.SetRetentionDays(store, 7)
	story, _ := deleteTree(t, svc)
	if err := svc.Delete(t.Context(), store, story.ID); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		targets []items.BulkTarget
		want    items.BulkDeletePreview
	}{
		{"one store and a read-only one", []items.BulkTarget{{Store: store, ID: apptest.SampleTask}, {Store: store, ID: apptest.SampleEpic}, {Store: store3, ID: apptest.SampleTask}}, items.BulkDeletePreview{Stores: 1, RetentionDays: 7, ReadOnly: 1}},
		{"two stores", []items.BulkTarget{{Store: store, ID: apptest.SampleTask}, {Store: store2, ID: apptest.SampleTask}}, items.BulkDeletePreview{Stores: 2}},
		{"only deleted", []items.BulkTarget{{Store: store, ID: story.ID}}, items.BulkDeletePreview{}},
	}
	for _, c := range cases {
		if got, err := svc.BulkDeletePreview(t.Context(), c.targets); err != nil || got != c.want {
			t.Errorf("%s: %+v, %v, want %+v", c.name, got, err, c.want)
		}
	}
}
