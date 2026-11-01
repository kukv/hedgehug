package gitrepo

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// A reader must always find the branch while a writer moves it: go-git
// truncates the reference file before it writes it, and a reader that opens
// the file in between used to see "reference not found" (and Head used to
// answer "no commit yet").
func TestHeadIsAlwaysReadableWhileCommitting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "0")
	if _, err := w.Commit(Commit{Paths: []string{"a.md"}, Subject: "0", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	reader, err := Open(dir) // another process's view of the same directory
	if err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	var wg sync.WaitGroup
	var bad atomic.Int64
	var first atomic.Value
	fail := func(err error) {
		bad.Add(1)
		first.CompareAndSwap(nil, err.Error())
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				if h, err := reader.Head(); err != nil || h.IsZero() {
					fail(fmt.Errorf("Head = %s, %v", h, err))
				}
				if _, err := reader.HeadTree(); err != nil {
					fail(err)
				}
			}
		}()
	}
	for i := 1; i <= 50; i++ {
		writeFile(t, dir, "a.md", fmt.Sprint(i))
		_, err := w.Commit(Commit{Paths: []string{"a.md"}, Subject: fmt.Sprint(i), Author: author, When: time.Now()})
		if err != nil && !errors.Is(err, ErrNothingToCommit) {
			t.Fatal(err)
		}
	}
	// Widen the window: the last thing a commit does, many times over.
	h, err := w.Head()
	if err != nil {
		t.Fatal(err)
	}
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)
	for range 5000 {
		if err := w.r.Storer.SetReference(ref); err != nil {
			t.Fatal(err)
		}
	}
	stop.Store(true)
	wg.Wait()
	if n := bad.Load(); n > 0 {
		t.Errorf("a reader failed %d times; first: %v", n, first.Load())
	}
}
