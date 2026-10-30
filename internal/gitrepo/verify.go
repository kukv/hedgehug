package gitrepo

import (
	"errors"
	"fmt"
	"io"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Verify reads every commit reachable from HEAD, with its trees and blobs,
// to the end. The first object that cannot be read is returned in the error
// with its hash. A repository without commits is sound.
func (r *Repo) Verify() error {
	it, err := r.r.Log(&git.LogOptions{})
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	defer it.Close()
	seen := map[plumbing.Hash]bool{}
	return it.ForEach(func(c *object.Commit) error {
		if err := r.readObject(c.Hash, seen); err != nil {
			return err
		}
		return r.verifyTree(c.TreeHash, seen)
	})
}

func (r *Repo) verifyTree(h plumbing.Hash, seen map[plumbing.Hash]bool) error {
	if seen[h] {
		return nil
	}
	if err := r.readObject(h, seen); err != nil {
		return err
	}
	t, err := r.r.TreeObject(h)
	if err != nil {
		return fmt.Errorf("object %s: %w", h, err)
	}
	for _, e := range t.Entries {
		switch e.Mode {
		case filemode.Dir:
			if err := r.verifyTree(e.Hash, seen); err != nil {
				return err
			}
		case filemode.Submodule:
			// The commit lives in another repository.
		default:
			if err := r.readObject(e.Hash, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

// readObject reads the object h to the end, once.
func (r *Repo) readObject(h plumbing.Hash, seen map[plumbing.Hash]bool) error {
	if seen[h] {
		return nil
	}
	o, err := r.r.Storer.EncodedObject(plumbing.AnyObject, h)
	if err != nil {
		return fmt.Errorf("object %s: %w", h, err)
	}
	rd, err := o.Reader()
	if err != nil {
		return fmt.Errorf("object %s: %w", h, err)
	}
	_, err = io.Copy(io.Discard, rd)
	if cerr := rd.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("object %s: %w", h, err)
	}
	seen[h] = true
	return nil
}
