package store

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

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

func validLocalName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid local file name %q", name)
	}
	return nil
}

// Repo returns the git repository of the store, for reading its history.
func (s *Store) Repo() *gitrepo.Repo { return s.repo }
