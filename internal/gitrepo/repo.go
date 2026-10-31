// Package gitrepo commits the files of a store with go-git. It knows nothing
// of the file formats; callers name the paths to commit.
package gitrepo

import (
	"fmt"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// Repo is a git repository with a work tree.
type Repo struct {
	dir string
	r   *git.Repository

	conn Conn // how Fetch and Push reach a remote; set by SetConn

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
		r.Storer = newCachedStorage(s)
	}
	return &Repo{dir: dir, r: r}
}
