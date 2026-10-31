package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/schema"
)

// ErrHeadMoved means HEAD was not the commit a merge was computed from, so
// the caller computes the merge again.
var ErrHeadMoved = errors.New("HEAD moved while the merge was computed")

// lockedAtHead takes the store lock, checks that HEAD is head and runs fn.
func (s *Store) lockedAtHead(head plumbing.Hash, fn func() error) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := acquire(s.path(lockPath), s.lockWait)
	if err != nil {
		return err
	}
	// As in Update, a release error only matters when the operation failed.
	defer func() {
		if rerr := l.release(); rerr != nil && err != nil {
			err = errors.Join(err, rerr)
		}
	}()
	cur, err := s.repo.Head()
	if err != nil {
		return err
	}
	if cur != head {
		return ErrHeadMoved
	}
	return fn()
}

// CommitMerge writes files (nil removes a file) and commits them with the
// given parents, under the store lock. It checks nothing and numbers nothing:
// a merge keeps what the sides made, and repair has already settled ids. It
// returns ErrHeadMoved when HEAD is not head, and gitrepo.ErrWorkTreeChanged
// when a file to write is no longer as in HEAD. trailers follow Hedgehug-Op and
// Hedgehug-Author. When the commit fails the files are put back.
func (s *Store) CommitMerge(head plumbing.Hash, parents []plumbing.Hash, files map[string][]byte, op Op, trailers []gitrepo.Trailer) (h plumbing.Hash, err error) {
	if op.Name == "" {
		return plumbing.ZeroHash, errors.New("the operation has no name")
	}
	if len(parents) == 0 || parents[0] != head {
		return plumbing.ZeroHash, errors.New("the first parent of a merge must be HEAD")
	}
	err = s.lockedAtHead(head, func() error {
		rels := make([]string, 0, len(files))
		for rel := range files {
			rels = append(rels, rel)
		}
		slices.Sort(rels)
		changes := make([]change, 0, len(rels))
		for _, rel := range rels {
			old, err := atomicfile.ReadFile(s.path(rel))
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			// A file saved by hand since the caller looked would be lost.
			was, herr := s.repo.ReadAt(head, rel)
			if herr != nil && !errors.Is(herr, fs.ErrNotExist) {
				return herr
			}
			if (err == nil) != (herr == nil) || !bytes.Equal(old, was) {
				return fmt.Errorf("%w: %s differs from HEAD", gitrepo.ErrWorkTreeChanged, rel)
			}
			changes = append(changes, change{rel: rel, data: files[rel], old: old, existed: err == nil})
		}
		subject := "hedgehug: " + op.Name
		if op.Detail != "" {
			subject += " " + op.Detail
		}
		ts := []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: op.Name}}
		if op.Account != "" {
			ts = append(ts, gitrepo.Trailer{Key: "Hedgehug-Author", Value: op.Account})
		}
		c := gitrepo.Commit{
			Paths:    rels,
			Parents:  parents,
			Subject:  subject,
			Trailers: append(ts, trailers...),
			Author:   op.Author,
			When:     s.now().Truncate(time.Second),
		}
		if err := s.apply(changes); err != nil {
			return err
		}
		if err := s.commit(c); err != nil {
			return errors.Join(err, s.rollback(changes))
		}
		if h, err = s.repo.Head(); err != nil {
			return err
		}
		s.refreshSchema(rels)
		return nil
	})
	return h, err
}

// FastForward moves HEAD to the commit to and writes paths from it, under the
// store lock. It returns ErrHeadMoved when HEAD is not head, and
// gitrepo.ErrWorkTreeChanged as Repo.FastForward does.
func (s *Store) FastForward(head, to plumbing.Hash, paths []string) error {
	return s.lockedAtHead(head, func() error {
		if err := s.repo.FastForward(to, paths); err != nil {
			return err
		}
		s.refreshSchema(paths)
		return nil
	})
}

// refreshSchema reads schema.yaml again when rels names it, so the cached
// schema is the one on disk. The commit or fast-forward has already happened,
// so a schema that does not decode is not an error here: the cache is left as
// it was and the next Update reads the file again and reports the problem.
func (s *Store) refreshSchema(rels []string) {
	if !slices.Contains(rels, schemaPath) {
		return
	}
	if sc, err := readDecode(s.dir, schemaPath, schema.Decode); err == nil {
		s.schema.Store(sc)
	}
}
