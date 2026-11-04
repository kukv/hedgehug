package osfs

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ErrLocked means another process kept a lock for longer than it was waited
// for. The text is the one the store has always shown: the store's own
// ErrLocked is this error.
var ErrLocked = errors.New("the store is locked by another process")

// defaultPoll is how often WithFileLock tries a taken lock again.
const defaultPoll = 50 * time.Millisecond

// Lock is an exclusive OS lock on a file. The lock belongs to the file
// handle, so two handles in one process exclude each other, and the OS drops
// it when the process exits.
type Lock struct{ f *os.File }

// LockFile takes the lock on the file at path, made when missing, in a
// directory that exists. While another holder has it, it tries again every
// poll for up to wait, calling onMiss (when not nil) after each try, and then
// fails with ErrLocked.
func LockFile(path string, wait, poll time.Duration, onMiss func()) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // G304: callers build paths from a store or a data directory
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
			return &Lock{f}, nil
		}
		if onMiss != nil {
			onMiss()
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, ErrLocked
		}
		time.Sleep(min(poll, time.Until(deadline)))
	}
}

// Holds reports whether the locked file is still the file at path: false
// once that file was removed or replaced.
func (l *Lock) Holds(path string) bool {
	a, err := l.f.Stat()
	if err != nil {
		return false
	}
	b, err := os.Stat(path)
	return err == nil && os.SameFile(a, b)
}

// Release drops the lock and closes its file.
func (l *Lock) Release() error {
	err := unlock(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}

// WithFileLock runs fn holding the OS lock on the file path, made with its
// directory when missing, waiting up to wait for another holder. It is for
// locks outside a store, such as the one that keeps two processes from
// refreshing a token at once.
func WithFileLock(path string, wait time.Duration, fn func() error) (err error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	l, err := LockFile(path, wait, defaultPoll, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rerr := l.Release(); err == nil {
			err = rerr
		}
	}()
	return fn()
}
