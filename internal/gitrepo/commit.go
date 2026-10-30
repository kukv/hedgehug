package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ErrNothingToCommit means a commit named no paths.
var ErrNothingToCommit = errors.New("nothing to commit")

// Signature is the author of a commit. The committer is the same.
type Signature struct{ Name, Email string }

// Trailer is one "Key: Value" line at the end of a commit message.
type Trailer struct{ Key, Value string }

// Commit describes one commit.
type Commit struct {
	Paths    []string // relative to the repository root, slash-separated
	Subject  string
	Trailers []Trailer
	Author   Signature
	When     time.Time
}

// Commit stages exactly c.Paths and commits the index. A path that is on
// disk is added; one that is not is dropped from the index.
//
// Paths are staged one by one so that a commit costs the same however many
// files the store holds: go-git's plain Add takes the status of the whole
// work tree.
func (r *Repo) Commit(c Commit) (plumbing.Hash, error) {
	if len(c.Paths) == 0 {
		return plumbing.ZeroHash, ErrNothingToCommit
	}
	w, err := r.r.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	for _, p := range c.Paths {
		_, err := os.Lstat(filepath.Join(r.dir, filepath.FromSlash(p)))
		switch {
		case err == nil:
			// SkipStatus is ignored for a missing path, which is why removals
			// go through unstage instead.
			if err := w.AddWithOptions(&git.AddOptions{Path: p, SkipStatus: true}); err != nil {
				return plumbing.ZeroHash, fmt.Errorf("add %s: %w", p, err)
			}
		case errors.Is(err, fs.ErrNotExist):
			if err := r.unstage(p); err != nil {
				return plumbing.ZeroHash, fmt.Errorf("remove %s: %w", p, err)
			}
		default:
			return plumbing.ZeroHash, err
		}
	}
	sig := &object.Signature{Name: c.Author.Name, Email: c.Author.Email, When: c.When}
	return w.Commit(message(c), &git.CommitOptions{Author: sig, Committer: sig})
}

// unstage drops p from the index without taking the status of the work tree.
func (r *Repo) unstage(p string) error {
	idx, err := r.r.Storer.Index()
	if err != nil {
		return err
	}
	if _, err := idx.Remove(p); err != nil {
		if errors.Is(err, index.ErrEntryNotFound) {
			return nil
		}
		return err
	}
	return r.r.Storer.SetIndex(idx)
}

func message(c Commit) string {
	var b strings.Builder
	b.WriteString(c.Subject)
	b.WriteString("\n")
	if len(c.Trailers) > 0 {
		b.WriteString("\n")
		for _, t := range c.Trailers {
			fmt.Fprintf(&b, "%s: %s\n", t.Key, t.Value)
		}
	}
	return b.String()
}
