package store

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/gitrepo"
)

// localDir holds what belongs to this machine only, like the lock and the
// index. git ignores it.
const localDir = ".hedgehug/local"

// ReadLocal reads the file name of .hedgehug/local. A missing file is
// ErrNotFound.
func (s *Store) ReadLocal(name string) ([]byte, error) {
	if err := validLocalName(name); err != nil {
		return nil, err
	}
	rel := localDir + "/" + name
	b, err := atomicfile.ReadFile(s.path(rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", rel, ErrNotFound)
	}
	return b, err
}

// WriteLocal replaces the file name of .hedgehug/local with data, through a
// temporary file and a rename. It takes no lock: what is kept here is a
// cache that any writer may replace.
func (s *Store) WriteLocal(name string, data []byte) error {
	if err := validLocalName(name); err != nil {
		return err
	}
	return atomicfile.WriteFile(s.path(localDir+"/"+name), data)
}

// WithLocalLock runs fn holding the file lock .hedgehug/local/<name>,
// waiting up to wait for it; ErrLocked when it stays taken. It is apart
// from the store lock, for what only this machine serializes, like syncs.
func (s *Store) WithLocalLock(name string, wait time.Duration, fn func() error) (err error) {
	if err := validLocalName(name); err != nil {
		return err
	}
	l, err := acquire(s.path(localDir+"/"+name), wait)
	if err != nil {
		return err
	}
	// As in Update, a release error only matters when fn failed.
	defer func() {
		if rerr := l.release(); rerr != nil && err != nil {
			err = errors.Join(err, rerr)
		}
	}()
	return fn()
}

func validLocalName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid local file name %q", name)
	}
	return nil
}

// Repo returns the git repository of the store, for reading its history.
func (s *Store) Repo() *gitrepo.Repo { return s.repo }

// Dir returns the directory of the store's work tree.
func (s *Store) Dir() string { return s.dir }
