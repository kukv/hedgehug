package statedb

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func recentItems(t *testing.T, r port.RecentItems) []port.Recent {
	t.Helper()
	got, err := r.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func mustTouch(t *testing.T, r port.RecentItems, store port.StoreID, item change.ID, at time.Time, keep int) {
	t.Helper()
	if err := r.Touch(t.Context(), store, item, at, keep); err != nil {
		t.Fatal(err)
	}
}

func TestRecentTouchKeepsNewestAndMovesToTop(t *testing.T) {
	r := openTemp(t).Recent()
	base := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	for i, id := range []change.ID{"a", "b", "c"} {
		if err := r.Touch(t.Context(), "S1", id, base.Add(time.Duration(i)*time.Minute), 2); err != nil {
			t.Fatal(err)
		}
	}
	got := recentItems(t, r)
	if len(got) != 2 || got[0].Item != "c" || got[1].Item != "b" {
		t.Fatalf("after a,b,c keep 2: %v", got)
	}
	if !got[0].At.Equal(base.Add(2 * time.Minute)) {
		t.Errorf("At roundtrip: %v", got[0].At)
	}
	if err := r.Touch(t.Context(), "S1", "b", base.Add(10*time.Minute), 2); err != nil {
		t.Fatal(err)
	}
	got = recentItems(t, r)
	if len(got) != 2 || got[0].Item != "b" || got[1].Item != "c" {
		t.Fatalf("after touching b: %v", got)
	}
}

// 時刻の文字列は小数部の桁数が違っても、新しさで並ぶ。
func TestRecentOrdersByTimeNotText(t *testing.T) {
	r := openTemp(t).Recent()
	base := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	mustTouch(t, r, "S1", "a", base.Add(500*time.Millisecond), 5)
	mustTouch(t, r, "S1", "b", base.Add(time.Second), 5)
	got := recentItems(t, r)
	if len(got) != 2 || got[0].Item != "b" {
		t.Fatalf("got %v", got)
	}
}

func TestRecentTouchUpdatesStore(t *testing.T) {
	r := openTemp(t).Recent()
	base := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	mustTouch(t, r, "S1", "a", base, 5)
	mustTouch(t, r, "S2", "a", base.Add(time.Minute), 5)
	got := recentItems(t, r)
	if len(got) != 1 || got[0].Store != "S2" {
		t.Fatalf("got %v", got)
	}
}

func TestRecentForgetRemovesRows(t *testing.T) {
	r := openTemp(t).Recent()
	base := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	for i, id := range []change.ID{"a", "b", "c"} {
		mustTouch(t, r, "S1", id, base.Add(time.Duration(i)*time.Minute), 5)
	}
	if err := r.Forget(t.Context(), []change.ID{"a", "c", "zzz"}); err != nil {
		t.Fatal(err)
	}
	got := recentItems(t, r)
	if len(got) != 1 || got[0].Item != "b" {
		t.Fatalf("got %v", got)
	}
	if err := r.Forget(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
}
