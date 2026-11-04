package browse_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

func TestLogViewTrailers(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	r, err := tasks.Add(ts.Session, tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	lr, err := browse.LogIn(ts.Session, nil, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	views := browse.StoreLogViews(lr.Logs)
	if len(views) != 1 {
		t.Fatalf("log = %d commits", len(views))
	}
	v := views[0]
	if !slices.Contains(v.Trailers, usecase.LogTrailer{Key: "Hedgehug-Op", Value: "add"}) ||
		!slices.Contains(v.Trailers, usecase.LogTrailer{Key: "Hedgehug-Task", Value: r.ULID.String()}) {
		t.Errorf("trailers = %+v; want Hedgehug-Op: add and Hedgehug-Task", v.Trailers)
	}
	// every trailer of the commit, in the order it is written
	want := lr.Logs[0].Entry.Trailers
	if len(v.Trailers) != len(want) {
		t.Fatalf("trailers = %+v; want the %d of the commit", v.Trailers, len(want))
	}
	for i, tr := range want {
		if v.Trailers[i].Key != tr.Key || v.Trailers[i].Value != tr.Value {
			t.Errorf("trailers[%d] = %+v; want %+v", i, v.Trailers[i], tr)
		}
	}

	// a commit with no trailers has [], not null
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if got := browse.LogViewOf(ix, gitrepo.LogEntry{}).Trailers; got == nil || len(got) != 0 {
		t.Errorf("trailers of a bare commit = %#v; want []", got)
	}
}
