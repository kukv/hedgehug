package gitrepo

import (
	"reflect"
	"testing"
	"time"
)

func TestStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "one")
	writeFile(t, dir, "b.md", "one")
	if _, err := r.Commit(Commit{Paths: []string{"a.md", "b.md"}, Subject: "init", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Status()
	if err != nil || len(got) != 0 {
		t.Fatalf("clean Status = %v, %v", got, err)
	}

	writeFile(t, dir, "a.md", "changed")   // unstaged
	writeFile(t, dir, "b.md", "changed")   // staged
	writeFile(t, dir, "c.md", "untracked") // unstaged
	writeFile(t, dir, ".gitignore", "ignored.md\n")
	writeFile(t, dir, "ignored.md", "x")
	w, err := r.r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("b.md"); err != nil {
		t.Fatal(err)
	}

	got, err = r.Status()
	if err != nil {
		t.Fatal(err)
	}
	want := []FileStatus{
		{Path: ".gitignore", Unstaged: true},
		{Path: "a.md", Unstaged: true},
		{Path: "b.md", Staged: true},
		{Path: "c.md", Unstaged: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Status = %+v, want %+v", got, want)
	}
}
