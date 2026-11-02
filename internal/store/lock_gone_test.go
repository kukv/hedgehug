package store

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func fakeStoreDir(t *testing.T) (dir, lock string) {
	t.Helper()
	dir = t.TempDir()
	for _, d := range []string{".git", ".hedgehug"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return dir, filepath.Join(dir, ".hedgehug", "local", "lock")
}

// A store whose directory is gone is not locked, and nothing is created in
// its place.
func TestAcquireStoreOfADirectoryThatIsGone(t *testing.T) {
	t.Parallel()
	dir, lock := fakeStoreDir(t)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireStore(dir, lock, time.Second); !errors.Is(err, ErrStoreGone) {
		t.Fatalf("err = %v; want ErrStoreGone", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the directory was created again: %v", err)
	}
}

// Serial: it replaces the package variable afterLockMiss.
//
// A writer that waits for the lock while the store is removed does not get
// the lock of the file that is gone.
func TestAcquireStoreWaitingWhileTheLockFileIsRemoved(t *testing.T) {
	dir, lock := fakeStoreDir(t)
	held, err := acquireStore(dir, lock, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	blocked := make(chan struct{})
	var once sync.Once
	afterLockMiss = func() { once.Do(func() { close(blocked) }) }
	t.Cleanup(func() { afterLockMiss = func() {} })
	errc := make(chan error, 1)
	go func() {
		l, err := acquireStore(dir, lock, 5*time.Second)
		if err == nil {
			_ = l.release()
		}
		errc <- err
	}()
	<-blocked // the waiter has the old file open and found it locked
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := held.release(); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; !errors.Is(err, ErrStoreGone) {
		t.Fatalf("waiter: err = %v; want ErrStoreGone", err)
	}
}

// A store whose .hedgehug directory is gone, as when it is removed between
// the check and the lock, is not made again.
func TestAcquireStoreMakesNoDirectoryOfARemovedStore(t *testing.T) {
	t.Parallel()
	dir, lock := fakeStoreDir(t)
	if err := os.RemoveAll(filepath.Join(dir, ".hedgehug")); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireStore(dir, lock, time.Second); !errors.Is(err, ErrStoreGone) {
		t.Fatalf("err = %v; want ErrStoreGone", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".hedgehug")); !os.IsNotExist(err) {
		t.Errorf(".hedgehug was made again: %v", err)
	}
}

// The unlocked writes of .hedgehug/local make the directories below
// .hedgehug but never .hedgehug itself.
func TestWriteLocalMakesNoDirectoryOfARemovedStore(t *testing.T) {
	t.Parallel()
	dir, _ := fakeStoreDir(t)
	if err := WriteLocalIn(dir, "overwritten/x.json", []byte("{}")); err != nil {
		t.Fatalf("with .hedgehug: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".hedgehug")); err != nil {
		t.Fatal(err)
	}
	if err := WriteLocalIn(dir, "index.json", []byte("{}")); !errors.Is(err, ErrStoreGone) {
		t.Fatalf("err = %v; want ErrStoreGone", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".hedgehug")); !os.IsNotExist(err) {
		t.Errorf(".hedgehug was made again: %v", err)
	}
}

func TestAcquireStoreHappyPath(t *testing.T) {
	t.Parallel()
	dir, lock := fakeStoreDir(t)
	l, err := acquireStore(dir, lock, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.release(); err != nil {
		t.Fatal(err)
	}
}
