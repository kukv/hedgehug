package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/gitrepo"
)

// LocalDir holds what belongs to this machine only, like the lock and the
// index. git ignores it.
const LocalDir = ".hedgehug/local"

// ReadLocal reads the file name of .hedgehug/local; a name may go into a
// directory, as in overwritten/<ULID>.json. A missing file is ErrNotFound
// and fs.ErrNotExist.
func (s *Store) ReadLocal(name string) ([]byte, error) {
	if err := validLocalName(name); err != nil {
		return nil, err
	}
	rel := LocalDir + "/" + name
	b, err := atomicfile.ReadFile(s.path(rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w: %w", rel, ErrNotFound, fs.ErrNotExist)
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
	return atomicfile.WriteFile(s.path(LocalDir+"/"+name), data)
}

// WriteLocalIn is WriteLocal for the store in dir, which it does not open: the
// merge driver runs in the middle of a merge, when the files of the store may
// not read.
func WriteLocalIn(dir, name string, data []byte) error {
	if err := validLocalName(name); err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(dir, filepath.FromSlash(LocalDir+"/"+name)), data)
}

// WithLocalLock runs fn holding the file lock .hedgehug/local/<name>,
// waiting up to wait for it; ErrLocked when it stays taken. It is apart
// from the store lock, for what only this machine serializes, like syncs.
func (s *Store) WithLocalLock(name string, wait time.Duration, fn func() error) (err error) {
	if err := validLocalName(name); err != nil {
		return err
	}
	l, err := acquire(s.path(LocalDir+"/"+name), wait)
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

// LocalFiles returns the names of the files in the directory dir of
// .hedgehug/local, sorted. A missing directory has none. Subdirectories and
// the temporary files of a write in progress, whose names start with a dot,
// are left out.
func (s *Store) LocalFiles(dir string) ([]string, error) {
	if err := validLocalName(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.path(LocalDir + "/" + dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries { // ReadDir sorts by name
		if e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// RemoveLocal removes the file name of .hedgehug/local. A missing file is
// not an error.
func (s *Store) RemoveLocal(name string) error {
	if err := validLocalName(name); err != nil {
		return err
	}
	err := os.Remove(s.path(LocalDir + "/" + name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// validLocalName accepts a name of slash-separated parts, none of them
// empty, . or .., so that it stays inside .hedgehug/local.
func validLocalName(name string) error {
	if strings.Contains(name, `\`) || slices.ContainsFunc(strings.Split(name, "/"), func(p string) bool { return p == "" || p == "." || p == ".." }) {
		return fmt.Errorf("invalid local file name %q", name)
	}
	return nil
}

// Repo returns the git repository of the store, for reading its history.
func (s *Store) Repo() *gitrepo.Repo { return s.repo }

// Dir returns the directory of the store's work tree.
func (s *Store) Dir() string { return s.dir }
