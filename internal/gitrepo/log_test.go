package gitrepo

import (
	"slices"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
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

	all, err := r.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Subject != "third" || all[2].Subject != "first" {
		t.Fatalf("Log(nil, 0) = %+v", all)
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

	for _, tc := range []struct {
		name     string
		prefixes []string
		limit    int
		want     int
	}{
		{"limit", nil, 2, 2},
		{"file", []string{"a.md"}, 0, 2},
		{"dir", []string{"d/"}, 0, 1},
		{"either", []string{"a.md", "d/"}, 0, 3},
	} {
		got, err := r.Log(tc.prefixes, tc.limit)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(got) != tc.want {
			t.Errorf("%s: len = %d, want %d", tc.name, len(got), tc.want)
		}
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
	last, err := r.Log(nil, 1)
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
	r, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Log(nil, 0)
	if err != nil || len(got) != 0 {
		t.Errorf("Log = %v, %v", got, err)
	}
}

func TestLogNoTrailers(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "x")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "only: subject", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Subject != "only: subject" || len(got[0].Trailers) != 0 {
		t.Errorf("entry = %+v", got[0])
	}
}
