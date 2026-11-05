package tasks_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// rankOrder is the IDs given, sorted as a list that crosses parents is: by
// rank, then by ULID.
func rankOrder(t *testing.T, ts *usecasetest.Session, ids ...string) string {
	t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ id, key string }
	var rows []row
	for _, id := range ids {
		e, err := ts.Resolve(ix, id)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row{id, string(e.Rank) + " " + e.ULID.String()})
	}
	slices.SortFunc(rows, func(a, b row) int { return strings.Compare(a.key, b.key) })
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.id
	}
	return strings.Join(out, " ")
}

// twoParents makes ME-1 and ME-2 with two children each: ME-3 and ME-4, ME-5
// and ME-6. The first children of the parents have one rank, and so have the
// second ones.
func twoParents(t *testing.T, ts *usecasetest.Session) {
	t.Helper()
	for _, in := range []tasks.AddInput{
		{Title: "p1"},
		{Title: "p2"},
		{Title: "a", Parent: "ME-1"},
		{Title: "b", Parent: "ME-1"},
		{Title: "c", Parent: "ME-2"},
		{Title: "d", Parent: "ME-2"},
	} {
		if _, err := tasks.Add(ts.Session, in); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRerankAcrossParents(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	twoParents(t, ts)
	// ME-3 and ME-5 share a rank, so ME-5 gets a new one too.
	res, err := tasks.Rerank(ts.Session, "ME-6", []string{"ME-3", "ME-6", "ME-5", "ME-4"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed {
		t.Error("nothing committed")
	}
	if got, want := rankOrder(t, ts, "ME-3", "ME-4", "ME-5", "ME-6"), "ME-3 ME-6 ME-5 ME-4"; got != want {
		t.Errorf("order = %s; want %s", got, want)
	}
	_, ix, _ := ts.Open()
	e, _ := ts.Resolve(ix, "ME-6")
	p, _ := ts.Resolve(ix, "ME-2")
	if e.Parent == nil || *e.Parent != p.ULID {
		t.Errorf("parent of ME-6 = %v; want ME-2", e.Parent)
	}
}

func TestRerankToTheEnds(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	twoParents(t, ts)
	if _, err := tasks.Rerank(ts.Session, "ME-6", []string{"ME-6", "ME-3", "ME-5", "ME-4"}); err != nil {
		t.Fatal(err)
	}
	if got, want := rankOrder(t, ts, "ME-3", "ME-4", "ME-5", "ME-6"), "ME-6 ME-3 ME-5 ME-4"; got != want {
		t.Errorf("first: order = %s; want %s", got, want)
	}
	if _, err := tasks.Rerank(ts.Session, "ME-6", []string{"ME-3", "ME-5", "ME-4", "ME-6"}); err != nil {
		t.Fatal(err)
	}
	if got, want := rankOrder(t, ts, "ME-3", "ME-4", "ME-5", "ME-6"), "ME-3 ME-5 ME-4 ME-6"; got != want {
		t.Errorf("last: order = %s; want %s", got, want)
	}
}

func TestRerankInPlaceCommitsNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	twoParents(t, ts)
	res, err := tasks.Rerank(ts.Session, "ME-2", strings.Fields(rankOrder(t, ts, "ME-1", "ME-2")))
	if err != nil {
		t.Fatal(err)
	}
	if res.Committed {
		t.Error("committed; want no commit")
	}
}

func TestRerankRefusals(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	twoParents(t, ts)
	if _, err := tasks.Remove(ts.Session, "ME-4"); err != nil {
		t.Fatal(err)
	}
	for name, order := range map[string][]string{
		"not in the order": {"ME-5", "ME-3"},
		"twice":            {"ME-6", "ME-6"},
		"deleted":          {"ME-6", "ME-4"},
	} {
		_, err := tasks.Rerank(ts.Session, "ME-6", order)
		if c := usecase.AsError(err); c.Code != "rank.invalid" {
			t.Errorf("%s: error = %v; want rank.invalid", name, err)
		}
	}
}
