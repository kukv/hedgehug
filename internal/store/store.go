// Package store reads and writes the files of one store, and commits every
// write to git as one commit per operation.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

const (
	schemaPath    = "schema.yaml"
	gitignorePath = ".gitignore"
	lockPath      = ".hedgehug/local/lock"
)

// ErrNotFound means a file the caller asked for is not in the store.
var ErrNotFound = errors.New("not found")

// Store is one store: a directory of task files under git.
type Store struct {
	dir  string
	repo *gitrepo.Repo

	mu     sync.Mutex // serializes Update within this Store
	meta   *model.StoreMeta
	schema atomic.Pointer[schema.Schema] // replaced by Update; read without mu

	idNumbers IDNumbers // counts id numbers for Update; nil reads every task

	// Replaced by tests.
	now      func() time.Time
	lockWait time.Duration
	commit   func(gitrepo.Commit) error
}

func newStore(dir string, repo *gitrepo.Repo, meta *model.StoreMeta, sc *schema.Schema) *Store {
	s := &Store{
		dir:      dir,
		repo:     repo,
		meta:     meta,
		now:      time.Now,
		lockWait: lockWait,
		commit: func(c gitrepo.Commit) error {
			_, err := repo.Commit(c)
			return err
		},
	}
	s.schema.Store(sc)
	return s
}

// newStoreHook lets tests change the store Create made before it writes.
var newStoreHook = func(*Store) {}

// Create makes a new store in dir, which must be missing or empty: a git
// repository holding store.yaml, sc as schema.yaml (the default schema when
// sc is nil), the workspace ws and a .gitignore, in one commit. meta and ws
// are copied, not changed.
// When Create fails it removes what it made, so it can be run again.
func Create(dir string, meta *model.StoreMeta, sc *schema.Schema, ws *model.Workspace, author gitrepo.Signature) (_ *Store, err error) {
	if ws == nil {
		return nil, errors.New("the store needs a workspace")
	}
	if ws.ULID.IsZero() {
		return nil, errors.New("the workspace has no ulid")
	}
	if err := model.ValidIDPrefix(ws.IDPrefix); err != nil {
		return nil, err
	}
	made, err := ensureEmpty(dir)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, undoCreate(dir, made))
		}
	}()
	repo, err := gitrepo.Init(dir)
	if err != nil {
		return nil, err
	}
	m := *meta
	m.Format = model.StoreFormat
	if sc == nil {
		sc = schema.Default()
	}
	s := newStore(dir, repo, &m, sc)
	newStoreHook(s)

	now := s.now().Truncate(time.Second)
	w := *ws
	for _, t := range []*time.Time{&w.Created, &w.Updated, &w.PrefixSet} {
		if t.IsZero() {
			*t = now
		}
	}

	mb, err := m.Encode()
	if err != nil {
		return nil, err
	}
	sb, err := s.Schema().Encode()
	if err != nil {
		return nil, err
	}
	wb, err := w.Encode()
	if err != nil {
		return nil, err
	}
	files := []struct {
		rel  string
		data []byte
	}{
		{model.StoreMetaPath, mb},
		{schemaPath, sb},
		{model.WorkspacePath(w.ULID), wb},
		{gitignorePath, []byte(".hedgehug/local/\n")},
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		if err := atomicfile.WriteFile(s.path(f.rel), f.data); err != nil {
			return nil, err
		}
		paths = append(paths, f.rel)
	}
	if err := s.commit(gitrepo.Commit{
		Paths:    paths,
		Subject:  "hedgehug: init",
		Trailers: []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "init"}},
		Author:   author,
		When:     now,
	}); err != nil {
		return nil, err
	}
	return s, nil
}

// ensureEmpty makes sure dir is an empty directory, creating it when it is
// missing. made tells whether it created it.
func ensureEmpty(dir string) (made bool, err error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return true, os.MkdirAll(dir, 0o750)
	}
	if err != nil {
		return false, err
	}
	if len(entries) > 0 {
		return false, fmt.Errorf("%s is not empty", dir)
	}
	return false, nil
}

// undoCreate removes what a failed Create left in dir: dir itself when
// Create made it, its contents otherwise.
func undoCreate(dir string, made bool) error {
	if made {
		return os.RemoveAll(dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		errs = append(errs, os.RemoveAll(filepath.Join(dir, e.Name())))
	}
	return errors.Join(errs...)
}

// Open opens the store in dir. It reads store.yaml and schema.yaml now;
// Meta returns what it read. Update reads schema.yaml again, and Schema
// returns the schema read last.
func Open(dir string) (*Store, error) {
	meta, err := readDecode(dir, model.StoreMetaPath, model.DecodeStoreMeta)
	if err != nil {
		return nil, err
	}
	sc, err := readDecode(dir, schemaPath, schema.Decode)
	if err != nil {
		return nil, err
	}
	repo, err := gitrepo.Open(dir)
	if err != nil {
		return nil, err
	}
	return newStore(dir, repo, meta, sc), nil
}

func (s *Store) Meta() *model.StoreMeta { return s.meta }
func (s *Store) Schema() *schema.Schema { return s.schema.Load() }

// path turns rel, slash-separated and relative to the store root, into a
// path on disk.
func (s *Store) path(rel string) string { return filepath.Join(s.dir, filepath.FromSlash(rel)) }
