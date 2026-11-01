package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

// A forward that reaches HEAD by a merge commit (the move-out commit is on the
// second parent) still gets the id the task had.
func TestForwardThroughAMergeKeepsTheID(t *testing.T) {
	s, dir := newStore(t)
	r := s.Repo()
	moved := putWithID(t, s, newTask("moved"))
	base, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	// the side: the move-out
	commitRaw(t, s, dir, model.TaskPath(moved.ULID), forwardOf(moved.ULID, destStore))
	side, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	// the main line: other work on the same base
	if err := r.ResetMain(base); err != nil {
		t.Fatal(err)
	}
	put(t, s, newTask("other"))
	mine, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	// the merge takes the forward from the second parent
	rel := model.TaskPath(moved.ULID)
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(forwardOf(moved.ULID, destStore)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "merge", Parents: []plumbing.Hash{mine, side}, Author: testAuthor, When: time.Now()}); err != nil {
		t.Fatal(err)
	}

	f, ok := mustOpen(t, s).Forward(moved.ULID)
	if !ok || f.ID != moved.ID || f.Broken != "" {
		t.Errorf("forward = %+v, %v; want id %s", f, ok, moved.ID)
	}
}
