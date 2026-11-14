package transfer_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/transfer"
)

func TestAliasesCollectsMovesOldestFirstThenOldPrefixes(t *testing.T) {
	at1 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	at2 := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	recs := []transfer.Record{
		{Entity: "item", Target: "x1", Old: "MB-4", New: "WEB-7", At: at2},
		{Entity: "item", Target: "x1", Old: "HH-12", New: "MB-4", At: at1},
		{Entity: "project", Target: "x1", Old: "HH-P1", At: at1},
		{Entity: "item", Target: "x2", Old: "HH-3", At: at1},
	}
	got := transfer.Aliases(recs, "item", "x1", []string{"W"}, 7, "-")
	want := []transfer.Alias{{ID: "HH-12", At: &at1}, {ID: "MB-4", At: &at2}, {ID: "W-7"}}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

func TestAliasesDropsDuplicates(t *testing.T) {
	at1 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	at2 := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	recs := []transfer.Record{
		{Entity: "item", Target: "x1", Old: "HH-12", At: at2},
		{Entity: "item", Target: "x1", Old: "HH-12", At: at1},
	}
	got := transfer.Aliases(recs, "item", "x1", nil, 1, "-")
	want := []transfer.Alias{{ID: "HH-12", At: &at1}}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

func TestAliasesProjectUsesPSeparator(t *testing.T) {
	got := transfer.Aliases(nil, "project", "p1", []string{"HH"}, 1, "-P")
	want := []transfer.Alias{{ID: "HH-P1"}}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}
