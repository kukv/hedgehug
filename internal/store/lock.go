package store

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ErrLocked means another process kept the store locked for longer than a
// write waits.
var ErrLocked = errors.New("the store is locked by another process")

// lockWait is how long a write waits for the lock, trying every lockPoll.
const lockWait = 10 * time.Second

var lockPoll = 50 * time.Millisecond

// fileLock is an exclusive OS lock on a file. The lock belongs to the file
// handle, so two handles in one process exclude each other, and the OS drops
// it when the process exits.
type fileLock struct{ f *os.File }

func acquire(path string, wait time.Duration) (*fileLock, error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //nolint:gosec // G703: paths are built from the store root
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // G703: paths are built from the store root
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		ok, err := tryLock(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		if ok {
			return &fileLock{f}, nil
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, ErrLocked
		}
		time.Sleep(min(lockPoll, time.Until(deadline)))
	}
}

// WithFileLock runs fn holding the OS lock on the file path, made with its
// directory when missing, waiting up to wait for another holder. It is for
// locks outside a store, such as the one that keeps two processes from
// refreshing a token at once.
func WithFileLock(path string, wait time.Duration, fn func() error) (err error) {
	l, err := acquire(path, wait)
	if err != nil {
		return err
	}
	defer func() {
		if rerr := l.release(); err == nil {
			err = rerr
		}
	}()
	return fn()
}

// ErrStoreGone means the directory of the store was removed, before the lock
// was taken or while it was waited for.
var ErrStoreGone = errors.New("the store directory is gone")

// acquireStore takes the lock file at path in the store at dir. It creates no
// directory for a store that is gone, and fails with ErrStoreGone when the
// store is removed while the lock is waited for: the lock file is then not
// the file at path any more, and a write would land in a deleted store.
func acquireStore(dir, path string, wait time.Duration) (*fileLock, error) {
	if !storeHere(dir) {
		return nil, ErrStoreGone
	}
	l, err := acquire(path, wait)
	if err != nil {
		return nil, err
	}
	if !storeHere(dir) || !sameFile(l.f, path) {
		_ = l.release()
		return nil, ErrStoreGone
	}
	return l, nil
}

func storeHere(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func sameFile(f *os.File, path string) bool {
	a, err := f.Stat()
	if err != nil {
		return false
	}
	b, err := os.Stat(path)
	return err == nil && os.SameFile(a, b)
}

func (l *fileLock) release() error {
	err := unlock(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}
