package gitrepo

import (
	"slices"
	"testing"
	"time"
)

// headLog is the first-parent log of HEAD, newest first.
func headLog(r *Repo, limit int) ([]LogEntry, error) {
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	return r.LogFrom(head, limit)
}

func TestLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	steps := []struct{ path, subject string }{{"a.md", "first"}, {"d/b.md", "second"}, {"a.md", "third"}}
	for i, s := range steps {
		writeFile(t, dir, s.path, s.subject)
		_, err := r.Commit(Commit{
			Paths: []string{s.path}, Subject: s.subject, Author: author,
			When:     base.Add(time.Duration(i) * time.Minute),
			Trailers: []Trailer{{"Hedgehug-Op", "add"}, {"Hedgehug-Task", "x"}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	all, err := headLog(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Subject != "third" || all[2].Subject != "first" {
		t.Fatalf("log = %+v", all)
	}
	if all[0].Author != author || !all[0].When.Equal(base.Add(2*time.Minute)) {
		t.Errorf("author/when = %+v %v", all[0].Author, all[0].When)
	}
	if v, ok := all[0].Trailer("Hedgehug-Op"); !ok || v != "add" {
		t.Errorf("Trailer = %q, %v", v, ok)
	}
	if _, ok := all[0].Trailer("Nope"); ok {
		t.Error("Trailer(Nope) found")
	}
	if len(all[0].Trailers) != 2 {
		t.Errorf("Trailers = %+v", all[0].Trailers)
	}

	got, err := headLog(r, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("limit: len = %d, want 2", len(got))
	}

	// A value may hold ": ", and a key may come more than once.
	writeFile(t, dir, "a.md", "fourth")
	_, err = r.Commit(Commit{
		Paths: []string{"a.md"}, Subject: "fourth", Author: author, When: base.Add(3 * time.Minute),
		Trailers: []Trailer{
			{"Hedgehug-Overwritten", `{"field":"a: b"}`},
			{"Hedgehug-Op", "merge"},
			{"Hedgehug-Overwritten", `{"field":"c"}`},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	last, err := headLog(r, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := last[0].TrailerValues("Hedgehug-Overwritten"); !slices.Equal(got, []string{`{"field":"a: b"}`, `{"field":"c"}`}) {
		t.Errorf("TrailerValues = %q", got)
	}
	if got := last[0].TrailerValues("Nope"); len(got) != 0 {
		t.Errorf("TrailerValues(Nope) = %q", got)
	}
}

func TestLogEmptyRepo(t *testing.T) {
	t.Parallel()
	r, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := headLog(r, 0)
	if err != nil || len(got) != 0 {
		t.Errorf("Log = %v, %v", got, err)
	}
}

func TestLogNoTrailers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "x")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "only: subject", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	got, err := headLog(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Subject != "only: subject" || len(got[0].Trailers) != 0 {
		t.Errorf("entry = %+v", got[0])
	}
}

func TestLogEntryWriter(t *testing.T) {
	t.Parallel()
	e := LogEntry{Author: Signature{Name: "a", Email: "a@x"}}
	if got := e.Writer(); got != "a@x" {
		t.Errorf("Writer = %q", got)
	}
	e.Trailers = []Trailer{{Key: "Hedgehug-Op", Value: "add"}, {Key: "Hedgehug-Author", Value: "github.com/a"}}
	if got := e.Writer(); got != "github.com/a" {
		t.Errorf("Writer = %q", got)
	}
}
