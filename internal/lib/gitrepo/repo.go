// Package gitrepo commits the files of a store with go-git. It knows nothing
// of the file formats; callers name the paths to commit.
package gitrepo

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// Repo is a git repository with a work tree.
type Repo struct {
	dir string
	r   *git.Repository

	conn Conn // how Fetch and Push reach a remote; set by SetConn

	closed atomic.Bool // Close was called; see Live

	mu    sync.Mutex
	sides map[[2]plumbing.Hash][]*object.Commit // side's results by from and base
}

// Init creates a repository in dir, with main as its default branch.
func Init(dir string) (*Repo, error) {
	r, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		return nil, fmt.Errorf("init %s: %w", dir, err)
	}
	return newRepo(dir, r), nil
}

// Open opens the repository in dir.
func Open(dir string) (*Repo, error) {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dir, err)
	}
	return newRepo(dir, r), nil
}

// newRepo wraps r, whose objects it reads through a cachedStorage.
func newRepo(dir string, r *git.Repository) *Repo {
	if s, ok := r.Storer.(*filesystem.Storage); ok {
		// Keep the packs open: go-git opens a pack for every object it reads
		// otherwise. Close lets go of them.
		s = filesystem.NewStorageWithOptions(s.Filesystem(), cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true})
		cs := newCachedStorage(s)
		cs.reader = newObjectReader(cs.gitDir)
		r.Storer = cs
	}
	liveMu.Lock()
	live[dir]++
	liveMu.Unlock()
	return &Repo{dir: dir, r: r}
}

// live counts the repositories made by dir that Close was not called on.
// Tests use it (Live) to check that whoever opened a repository closed it.
var (
	liveMu sync.Mutex
	live   = map[string]int{}
)

// Live is how many repositories in or under dir have not been closed. A
// handle that is read and reopened after Close was called on it (as
// Consolidate does when it closes the repository itself) is not counted.
func Live(dir string) int {
	liveMu.Lock()
	defer liveMu.Unlock()
	n := 0
	for d, c := range live {
		if d == dir || strings.HasPrefix(d, dir+string(os.PathSeparator)) {
			n += c
		}
	}
	return n
}

// Close lets go of the files the repository holds open (the object
// storage), so its directory can be renamed on Windows.
func (r *Repo) Close() error {
	if r.closed.CompareAndSwap(false, true) {
		liveMu.Lock()
		if live[r.dir]--; live[r.dir] == 0 {
			delete(live, r.dir) // the server opens stores for ever
		}
		liveMu.Unlock()
	}
	if c, ok := r.r.Storer.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
