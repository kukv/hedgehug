package gitrepo

import (
	"errors"
	"testing"
	"time"
)

func TestCommitAllowEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := commitHead(t, r, dir, map[string]string{"a.md": "a\n"})
	if _, err := r.Commit(Commit{Subject: "s", Author: author, When: time.Now()}); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("an empty commit without AllowEmpty = %v", err)
	}
	h, err := r.Commit(Commit{Subject: "empty", Trailers: []Trailer{{"Hedgehug-Undo", "x"}}, Author: author, When: time.Now(), AllowEmpty: true})
	if err != nil {
		t.Fatal(err)
	}
	log, err := r.LogFrom(h, 0)
	if err != nil || len(log) != 2 || log[0].Subject != "empty" || log[1].Hash != first {
		t.Fatalf("log = %+v, %v", log, err)
	}
	if v, _ := log[0].Trailer("Hedgehug-Undo"); v != "x" {
		t.Errorf("trailers = %v", log[0].Trailers)
	}
	// Paths whose files equal HEAD's make an empty commit as well.
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "same", Author: author, When: time.Now(), AllowEmpty: true}); err != nil {
		t.Errorf("same paths with AllowEmpty = %v", err)
	}
}

func TestResolveCommit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := commitHead(t, r, dir, map[string]string{"a.md": "a\n"})
	for _, rev := range []string{h.String(), h.String()[:7], "HEAD"} {
		if got, err := r.ResolveCommit(rev); err != nil || got != h {
			t.Errorf("ResolveCommit(%q) = %s, %v", rev, got, err)
		}
	}
	if _, err := r.ResolveCommit("nonsense"); err == nil {
		t.Error("ResolveCommit(nonsense) succeeded")
	}
}
