package gitrepo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ErrGitBusy means another process holds .git/index.lock.
var ErrGitBusy = errors.New("git is running in the store (.git/index.lock exists); try again when it ends, or remove the file if it is left over")

// indexLockWait is how long lockIndex waits for another process's
// .git/index.lock to go away, trying every indexLockPoll.
var indexLockWait = 2 * time.Second

var indexLockPoll = 100 * time.Millisecond

// lockIndex creates .git/index.lock, the lock git takes before it writes the
// index or HEAD, so that Hedgehug and a git run by hand do not write at the
// same time. go-git never takes it itself, so holding it does not stop
// go-git's own writes. unlock removes the file; a lock that could not be taken
// is ErrGitBusy and leaves the existing file alone.
func (r *Repo) lockIndex() (unlock func(), err error) {
	p := filepath.Join(r.dir, ".git", "index.lock")
	deadline := time.Now().Add(indexLockWait)
	for {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // G304: the path is built from the repository root and a fixed name
		if err == nil {
			return func() {
				_ = f.Close()
				_ = os.Remove(p)
			}, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, ErrGitBusy
		}
		time.Sleep(min(indexLockPoll, time.Until(deadline)))
	}
}
