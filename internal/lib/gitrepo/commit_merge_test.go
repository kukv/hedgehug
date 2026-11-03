package gitrepo

import (
	"errors"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestMergeCommitWithoutPaths(t *testing.T) {
	t.Parallel()
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	hb := commitHead(t, b.r, b.dir, map[string]string{"x.md": "a\n"})
	m, err := b.r.Commit(Commit{Subject: "merge", Parents: []plumbing.Hash{hb, theirs}, Author: author, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := b.r.Head(); got != m {
		t.Errorf("HEAD = %s, want %s", got, m)
	}
	if _, err := b.r.Commit(Commit{Subject: "s", Parents: []plumbing.Hash{m}, Author: author, When: time.Now()}); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("Commit with one parent and no path = %v", err)
	}
}
