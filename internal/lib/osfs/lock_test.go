package osfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWithFileLock(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "sub", "auth.lock")
	inside := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = WithFileLock(p, time.Second, func() error {
			close(inside)
			<-release
			return nil
		})
	}()
	<-inside
	if err := WithFileLock(p, 20*time.Millisecond, func() error { return nil }); !errors.Is(err, ErrLocked) {
		t.Errorf("second lock = %v, want ErrLocked", err)
	}
	close(release)
	want := errors.New("from fn")
	if err := WithFileLock(p, time.Second, func() error { return want }); !errors.Is(err, want) {
		t.Errorf("err = %v, want fn's", err)
	}
}

func TestLockFileCallsOnMiss(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "lock")
	l, err := LockFile(p, time.Second, time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	misses := 0
	if _, err := LockFile(p, 20*time.Millisecond, 5*time.Millisecond, func() { misses++ }); !errors.Is(err, ErrLocked) {
		t.Fatalf("second LockFile: err = %v, want ErrLocked", err)
	}
	if misses == 0 {
		t.Error("onMiss was not called while the lock was taken")
	}
}

func TestLockHolds(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "lock")
	l, err := LockFile(p, time.Second, time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	if !l.Holds(p) {
		t.Error("Holds = false for the file that was locked")
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if l.Holds(p) {
		t.Error("Holds = true after the file was removed")
	}
}
