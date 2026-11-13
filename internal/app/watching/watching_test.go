package watching_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/watching"
	"github.com/kukv/hedgehug/internal/domain/change"
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

func setup(t *testing.T, login string) (watching.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return watching.Service{Stores: rec, Identity: apptest.Identity{Login: login}}, rec
}

func watchers(t *testing.T, s port.Stores) []string {
	t.Helper()
	var out []string
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		it, err := r.Items().Get(apptest.SampleTask)
		out = it.Watchers
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func log(t *testing.T, s port.Stores) []change.Record {
	t.Helper()
	var out []change.Record
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		out, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStartAndStopWatching(t *testing.T) {
	svc, rec := setup(t, "hanako")
	// サンプルのタスクは taro がウォッチ中。
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil {
		t.Fatal(err)
	}
	if got := watchers(t, rec); !slices.Equal(got, []string{"hanako", "taro"}) {
		t.Errorf("watchers = %v", got)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 のウォッチを開始" {
		t.Errorf("summary = %q", got)
	}
	if err := svc.Set(t.Context(), store, apptest.SampleTask, false); err != nil {
		t.Fatal(err)
	}
	if got := watchers(t, rec); !slices.Equal(got, []string{"taro"}) {
		t.Errorf("watchers = %v", got)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 のウォッチを解除" {
		t.Errorf("summary = %q", got)
	}
}

func TestSettingTheStateItAlreadyHasSavesNothing(t *testing.T) {
	svc, rec := setup(t, "hanako")
	before := len(log(t, rec))
	if err := svc.Set(t.Context(), store, apptest.SampleTask, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil {
		t.Fatal(err)
	}
	if got := len(log(t, rec)); got != before+1 {
		t.Errorf("log rows %d → %d, want one new row (the first start only)", before, got)
	}
}

// ログイン名は大文字と小文字を区別せずに比べる。解除は、保存してある綴りで書く。
func TestLoginsAreComparedWithoutCase(t *testing.T) {
	svc, rec := setup(t, "hanako")
	err := rec.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "ウォッチを足す"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewAdd("watchers", "Hanako"))
	})
	if err != nil {
		t.Fatal(err)
	}
	before := len(log(t, rec))
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil || len(log(t, rec)) != before {
		t.Errorf("starting while watching as Hanako wrote a row (%v)", err)
	}
	if err := svc.Set(t.Context(), store, apptest.SampleTask, false); err != nil {
		t.Fatal(err)
	}
	rows := log(t, rec)
	if last := rows[len(rows)-1]; last.T != change.Remove || string(last.From) != `"Hanako"` {
		t.Errorf("last row = %+v, want a remove of the stored spelling", last)
	}
	if got := watchers(t, rec); !slices.Equal(got, []string{"taro"}) {
		t.Errorf("watchers = %v", got)
	}
}

func TestUnknownItem(t *testing.T) {
	svc, _ := setup(t, "hanako")
	if err := svc.Set(t.Context(), store, "01NOPE00000000000000000000", true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}
