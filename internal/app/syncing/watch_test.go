package syncing_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

// 06「画面への反映」: 版が変わった Store を、書き手を問わず知らせる。最初は覚えるだけ。
func TestWatchPublishesChangedStores(t *testing.T) {
	versions := &apptest.StoreVersions{Value: map[port.StoreID]string{"S1": "a", "S2": "b"}}
	events := &apptest.Events{}
	w := &syncing.Watch{Versions: versions, Events: events}
	ctx := context.Background()
	if err := w.Step(ctx); err != nil || len(events.List()) != 0 {
		t.Fatalf("first step: %v, %+v", err, events.List())
	}
	versions.Set(map[port.StoreID]string{"S1": "a2", "S3": "c"}) // S1 が変わり、S2 が消え、S3 が増えた
	if err := w.Step(ctx); err != nil {
		t.Fatal(err)
	}
	want := []port.Event{
		{Kind: port.EventStore, Store: "S1"},
		{Kind: port.EventStore, Store: "S2"},
		{Kind: port.EventStore, Store: "S3"},
	}
	if got := events.List(); !slices.Equal(got, want) {
		t.Errorf("events = %+v", got)
	}
	if err := w.Step(ctx); err != nil || len(events.List()) != 3 {
		t.Errorf("unchanged step published: %+v", events.List())
	}
}
