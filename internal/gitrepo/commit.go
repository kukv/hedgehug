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

// ErrNothingToCommit means a commit named no paths, or its paths are the
// same as in HEAD.
var ErrNothingToCommit = errors.New("nothing to commit")

// Signature is the author of a commit. The committer is the same.
type Signature struct{ Name, Email string }

// Trailer is one "Key: Value" line at the end of a commit message.
type Trailer struct{ Key, Value string }

// Commit describes one commit.
type Commit struct {
	Paths    []string        // relative to the repository root, slash-separated
	Parents  []plumbing.Hash // HEAD when empty
	Subject  string
	Trailers []Trailer
	Author   Signature
	When     time.Time
	// AllowEmpty makes the commit even when its tree is the same as the
	// first parent's, and without any path.
	AllowEmpty bool
}

// Commit stages exactly c.Paths and commits the index. A path that is on
// disk is added; one that is not is dropped from the index.
//
// Paths are staged one by one so that a commit costs the same however many
// files the store holds: go-git's plain Add takes the status of the whole
// work tree.
//
// If the commit fails, the index is put back as it was before the call, so
// that staged paths do not leak into the next commit.
//
// A commit with two or more parents, or with AllowEmpty, is made even when
// its tree is the same as the first parent's, and without any path: the
// parents or the caller alone make it a change.
func (r *Repo) Commit(c Commit) (h plumbing.Hash, err error) {
	if len(c.Paths) == 0 && len(c.Parents) < 2 && !c.AllowEmpty {
		return plumbing.ZeroHash, ErrNothingToCommit
	}
	unlock, err := r.lockIndex()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	defer unlock()
	before, err := r.r.Storer.Index()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	defer func() {
		if err != nil {
			if rerr := r.r.Storer.SetIndex(before); rerr != nil {
				err = errors.Join(err, rerr)
			}
		}
	}()
	w, err := r.r.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if err := r.stage(w, c.Paths); err != nil {
		return plumbing.ZeroHash, err
	}
	sig := &object.Signature{Name: c.Author.Name, Email: c.Author.Email, When: c.When}
	h, err = w.Commit(message(c), &git.CommitOptions{
		Author: sig, Committer: sig, Parents: c.Parents, AllowEmptyCommits: len(c.Parents) >= 2 || c.AllowEmpty,
	})
	if errors.Is(err, git.ErrEmptyCommit) {
		return plumbing.ZeroHash, ErrNothingToCommit
	}
	return h, err
}

// stage makes the index entries of paths match the work tree: a path that is
// on disk is added; one that is not is dropped from the index.
func (r *Repo) stage(w *git.Worktree, paths []string) error {
	for _, p := range paths {
		_, err := os.Lstat(filepath.Join(r.dir, filepath.FromSlash(p)))
		switch {
		case err == nil:
			// SkipStatus is ignored for a missing path, which is why removals
			// go through unstage instead.
			if err := w.AddWithOptions(&git.AddOptions{Path: p, SkipStatus: true}); err != nil {
				return fmt.Errorf("add %s: %w", p, err)
			}
		case errors.Is(err, fs.ErrNotExist):
			if err := r.unstage(p); err != nil {
				return fmt.Errorf("remove %s: %w", p, err)
			}
		default:
			return err
		}
	}
	return nil
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
