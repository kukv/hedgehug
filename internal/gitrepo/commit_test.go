package gitrepo

import (
	"errors"
	"testing"
	"time"
)

func TestCommitUnchangedPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "one")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "one", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_, err = r.Commit(Commit{Paths: []string{"a.md"}, Subject: "again", Author: author, When: time.Now()})
	if !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("err = %v, want ErrNothingToCommit", err)
	}
}
