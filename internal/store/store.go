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

// Create makes a new store in dir, which must be missing or empty: a git
// repository holding store.yaml, the default schema and a .gitignore, in
// one commit.
func Create(dir string, meta *model.StoreMeta, author gitrepo.Signature) (*Store, error) {
	if err := ensureEmpty(dir); err != nil {
		return nil, err
	}
	repo, err := gitrepo.Init(dir)
	if err != nil {
		return nil, err
	}
	meta.Format = model.StoreFormat
	s := newStore(dir, repo, meta, schema.Default())

	mb, err := meta.Encode()
	if err != nil {
		return nil, err
	}
	sb, err := s.Schema().Encode()
	if err != nil {
		return nil, err
	}
	files := []struct {
		rel  string
		data []byte
	}{
		{model.StoreMetaPath, mb},
		{schemaPath, sb},
		{gitignorePath, []byte(".hedgehug/local/\n")},
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		if err := writeFile(s.path(f.rel), f.data); err != nil {
			return nil, err
		}
		paths = append(paths, f.rel)
	}
	if err := s.commit(gitrepo.Commit{
		Paths:    paths,
		Subject:  "hedgehug: init",
		Trailers: []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "init"}},
		Author:   author,
		When:     s.now().Truncate(time.Second),
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func ensureEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(dir, 0o750)
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty", dir)
	}
	return nil
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
