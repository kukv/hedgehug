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
		time.Sleep(lockPoll)
	}
}

func (l *fileLock) release() error {
	err := unlock(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}
