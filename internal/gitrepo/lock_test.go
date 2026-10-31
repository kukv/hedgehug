package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCommitWaitsForIndexLock(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	indexLockWait = 50 * time.Millisecond
	t.Cleanup(func() { indexLockWait = 2 * time.Second })
	writeFile(t, dir, "a.md", "a\n")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "s", Author: Signature{"a", "a@x"}, When: time.Now()}); !errors.Is(err, ErrGitBusy) {
		t.Errorf("Commit with index.lock = %v", err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the index.lock of another process is gone: %v", err)
	}
	_ = os.Remove(lock)
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "s", Author: Signature{"a", "a@x"}, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Error("index.lock is left after the commit")
	}
}

func TestCommitTakesTheLockWhenItIsReleased(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(150 * time.Millisecond)
		_ = os.Remove(lock)
	}()
	writeFile(t, dir, "a.md", "a\n")
	_, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "s", Author: author, When: time.Now()})
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Error("index.lock is left after the commit")
	}
}

func TestFastForwardWaitsForIndexLock(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	before, _ := b.r.Head()
	lock := filepath.Join(b.dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	indexLockWait = 50 * time.Millisecond
	t.Cleanup(func() { indexLockWait = 2 * time.Second })
	if err := b.r.FastForward(theirs, []string{"x.md"}); !errors.Is(err, ErrGitBusy) {
		t.Errorf("FastForward with index.lock = %v", err)
	}
	if got, _ := b.r.Head(); got != before {
		t.Errorf("HEAD moved to %s", got)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "x.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("x.md was written: %v", err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the index.lock of another process is gone: %v", err)
	}
}

func TestRepack(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	for i := range 5 {
		commitFiles(t, r, dir, map[string]string{"a.md": fmt.Sprint(i)})
	}
	n, err := r.LooseObjects()
	if err != nil || n == 0 {
		t.Fatalf("LooseObjects = %d, %v", n, err)
	}
	if err := r.Repack(); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.LooseObjects(); n != 0 {
		t.Errorf("loose objects after repack = %d", n)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("Verify after repack: %v", err)
	}
}
