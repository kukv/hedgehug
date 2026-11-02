package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fakeStoreDir(t *testing.T) (dir, lock string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o750); err != nil {
		t.Fatal(err)
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

// A writer that waits for the lock while the store is removed does not get
// the lock of the file that is gone.
func TestAcquireStoreWaitingWhileTheLockFileIsRemoved(t *testing.T) {
	t.Parallel()
	dir, lock := fakeStoreDir(t)
	held, err := acquireStore(dir, lock, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	errc := make(chan error, 1)
	go func() {
		l, err := acquireStore(dir, lock, 5*time.Second)
		if err == nil {
			_ = l.release()
		}
		errc <- err
	}()
	time.Sleep(300 * time.Millisecond) // the waiter has the old file open and is polling
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
