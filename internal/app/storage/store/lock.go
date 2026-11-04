package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// ErrLocked means another process kept the store locked for longer than a
// write waits. It is osfs.ErrLocked, which osfs.WithFileLock also returns.
var ErrLocked = osfs.ErrLocked

// lockWait is how long a write waits for the lock, trying every lockPoll.
const lockWait = 10 * time.Second

var lockPoll = 50 * time.Millisecond

// afterLockMiss is called each time acquire finds the lock taken, with the
// lock file open; tests use it to know a waiter is blocked.
var afterLockMiss = func() {}

func acquire(path string, wait time.Duration) (*osfs.Lock, error) {
	path = filepath.Clean(path)
	if err := osfs.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return lockFile(path, wait)
}

// lockFile takes the lock on the file at path, made when missing, in a
// directory that exists.
func lockFile(path string, wait time.Duration) (*osfs.Lock, error) {
	return osfs.LockFile(path, wait, lockPoll, func() { afterLockMiss() })
}

// ErrStoreGone means the directory of the store was removed or replaced (by
// doctor --fix's swap, for one), before the lock was taken or while it was
// waited for.
var ErrStoreGone = errors.New("the store directory was removed or replaced; run the command again")

// acquireStore takes the lock file at path in the store at dir. It creates no
// directory above .hedgehug/local, so a store removed meanwhile is not made
// again, and fails with ErrStoreGone when the store is removed while the lock
// is waited for: the lock file is then not the file at path any more, and a
// write would land in a deleted store.
func acquireStore(dir, path string, wait time.Duration) (*osfs.Lock, error) {
	if !storeHere(dir) {
		return nil, ErrStoreGone
	}
	if err := mkdirInStore(dir, path); err != nil {
		return nil, err
	}
	l, err := lockFile(filepath.Clean(path), wait)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrStoreGone
	}
	if err != nil {
		return nil, err
	}
	if !storeHere(dir) || !l.Holds(path) {
		_ = l.Release()
		return nil, ErrStoreGone
	}
	return l, nil
}

// mkdirInStore makes the directories of path that are below the first
// directory of the store (.hedgehug), one level at a time with no MkdirAll: a
// missing parent means the store was removed, which is ErrStoreGone, and it is
// not made again.
func mkdirInStore(dir, path string) error {
	rel, err := filepath.Rel(dir, filepath.Dir(path))
	if err != nil {
		return err
	}
	cur := dir
	for i, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		if i == 0 {
			continue // .hedgehug holds a tracked file: a live store has it
		}
		if err := os.Mkdir(cur, 0o750); err != nil && !errors.Is(err, fs.ErrExist) { //nolint:gosec // G703: paths are built from the store root
			if errors.Is(err, fs.ErrNotExist) {
				return ErrStoreGone
			}
			return err
		}
	}
	return nil
}

func storeHere(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
