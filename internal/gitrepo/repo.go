// Package gitrepo commits the files of a store with go-git. It knows nothing
// of the file formats; callers name the paths to commit.
package gitrepo

import (
	"fmt"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Repo is a git repository with a work tree.
type Repo struct {
	dir string
	r   *git.Repository
}

// Init creates a repository in dir, with main as its default branch.
func Init(dir string) (*Repo, error) {
	r, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		return nil, fmt.Errorf("init %s: %w", dir, err)
	}
	return &Repo{dir: dir, r: r}, nil
}

// Open opens the repository in dir.
func Open(dir string) (*Repo, error) {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dir, err)
	}
	return &Repo{dir: dir, r: r}, nil
}
