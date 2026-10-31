package query

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
)

// sortFixture holds three tasks:
//
//	t1: rank a2, todo, due 2026-11-20, priority low, labels [x y]
//	t2: rank a0, in_progress, no due, priority urgent, labels [y]
//	t3: rank a1, done, due 2026-11-05, priority high, no labels
func sortFixture(t *testing.T) *index.Index {
	t.Helper()
	s := newStore(t)
	t1, t2, t3 := newTask("t1"), newTask("t2"), newTask("t3")
	t1.Rank, t2.Rank, t3.Rank = model.Rank("a2"), model.Rank("a0"), model.Rank("a1")
	t2.Status, t3.Status = "in_progress", "done"
	setProp(t1, "due", "2026-11-20")
	setProp(t3, "due", "2026-11-05")
	setProp(t1, "priority", "low")
	setProp(t2, "priority", "urgent")
	setProp(t3, "priority", "high")
	t1.Labels, t2.Labels = []string{"x", "y"}, []string{"y"}
	put(t, s, t1, t2, t3)
	return mustOpen(t, s)
}

func TestRunSort(t *testing.T) {
	ix := sortFixture(t)
	tests := []struct {
		q    string
		want []string
	}{
		{"", []string{"t2", "t3", "t1"}}, // rank
		{"sort:due", []string{"t3", "t1", "t2"}},
		{"sort:-due", []string{"t1", "t3", "t2"}}, // no value stays last
		{"sort:priority", []string{"t1", "t3", "t2"}},
		{"sort:-priority", []string{"t2", "t3", "t1"}},
		{"sort:title", []string{"t1", "t2", "t3"}},
		{"sort:status", []string{"t1", "t2", "t3"}},
	}
	for _, tt := range tests {
		got := run(t, ix, tt.q, Options{})
		if len(got) != 1 || !reflect.DeepEqual(got[0], tt.want) {
			t.Errorf("%q = %v, want [%v]", tt.q, got, tt.want)
		}
	}
}

func TestRunGroup(t *testing.T) {
	ix := sortFixture(t)
	tests := []struct {
		q        string
		wantKeys []string
		want     [][]string
	}{
		{"group:status", []string{"todo", "in_progress", "done"}, [][]string{{"t1"}, {"t2"}, {"t3"}}},
		{"group:label", []string{"x", "y", ""}, [][]string{{"t1"}, {"t2", "t1"}, {"t3"}}},
		{"group:priority sort:due", []string{"low", "high", "urgent"}, [][]string{{"t1"}, {"t3"}, {"t2"}}},
		{"group:due", []string{"2026-11-05", "2026-11-20", ""}, [][]string{{"t3"}, {"t1"}, {"t2"}}},
		{"group:status kind:bug", []string{}, [][]string{}},
	}
	for _, tt := range tests {
		q, err := Parse(tt.q)
		if err != nil {
			t.Fatal(err)
		}
		res, err := Run(ix, q, Options{})
		if err != nil {
			t.Fatalf("Run(%q): %v", tt.q, err)
		}
		keys := []string{}
		for _, g := range res.Groups {
			keys = append(keys, g.Key)
		}
		if !reflect.DeepEqual(keys, tt.wantKeys) {
			t.Errorf("%q keys = %v, want %v", tt.q, keys, tt.wantKeys)
		}
		if got := run(t, ix, tt.q, Options{}); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q = %v, want %v", tt.q, got, tt.want)
		}
	}
}

func TestRunSortErrors(t *testing.T) {
	ix := sortFixture(t)
	tests := []struct {
		q   string
		pos int
	}{
		{"sort:due sort:title", 2},
		{"group:status group:kind", 2},
		{"sort:label", 1},
		{"group:nope", 1},
		{"-sort:due", 1},
		{"sort<due", 1},
		{"sort:due,title", 1},
	}
	for _, tt := range tests {
		q, err := Parse(tt.q)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Run(ix, q, Options{})
		var qe *Error
		if !errors.As(err, &qe) || qe.Pos != tt.pos {
			t.Errorf("Run(%q) err = %v, want an Error at term %d", tt.q, err, tt.pos)
		}
	}
}

func TestRunRankTiesAreOrderedByID(t *testing.T) {
	s := newStore(t)
	// The ULID order (ten first) differs from the id order.
	ten, nine := newTask("ten"), newTask("nine")
	ten.ID, nine.ID = "TST-10", "TST-9"
	ten.Rank, nine.Rank = "a5", "a5"
	put(t, s, ten, nine)

	ix := mustOpen(t, s)
	if got := run(t, ix, "", Options{}); len(got) != 1 || !reflect.DeepEqual(got[0], []string{"nine", "ten"}) {
		t.Errorf("= %v, want [nine ten]", got)
	}
}
