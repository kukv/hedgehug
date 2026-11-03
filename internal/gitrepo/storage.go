package gitrepo

import (
	"errors"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// cachedStorage looks an object up in memory before it touches the disk.
// go-git opens the object file, or the packfile, before it looks in its
// cache, and on Windows each open and each stat of the path costs tens of
// microseconds. An object never changes under its hash, so the copy in
// memory is the object whether or not the file still exists.
//
// With a reader, objects are read by it and only those it cannot find by
// go-git (spec gogit-fixed-cost 2): go-git loads every pack index whole on
// its first read. Writes and walks over every object stay with go-git.
type cachedStorage struct {
	*filesystem.Storage
	objects cache.Object
	gitDir  string        // the git directory; references are written there (refs.go)
	reader  *objectReader // nil: go-git reads every object
}

func newCachedStorage(s *filesystem.Storage) *cachedStorage {
	return &cachedStorage{Storage: s, objects: cache.NewObjectLRUDefault(), gitDir: s.Filesystem().Root()}
}

func (s *cachedStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	if o, ok := s.objects.Get(h); ok && (t == plumbing.AnyObject || o.Type() == t) {
		return o, nil
	}
	o, err := s.read(t, h)
	if err != nil {
		return nil, err
	}
	s.objects.Put(o)
	return o, nil
}

// read reads h with the reader, and with go-git when the reader does not
// find it.
func (s *cachedStorage) read(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	if s.reader != nil {
		o, err := s.reader.object(h)
		switch {
		case err == nil:
			if t != plumbing.AnyObject && o.Type() != t {
				return nil, plumbing.ErrObjectNotFound
			}
			return o, nil
		case !errors.Is(err, plumbing.ErrObjectNotFound):
			return nil, err
		}
	}
	return s.Storage.EncodedObject(t, h)
}

func (s *cachedStorage) HasEncodedObject(h plumbing.Hash) error {
	if s.reader != nil {
		ok, err := s.reader.has(h)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return s.Storage.HasEncodedObject(h)
}

// Reindex makes go-git and the reader list the packs anew.
func (s *cachedStorage) Reindex() {
	s.Storage.Reindex()
	if s.reader != nil {
		s.reader.stale()
	}
}

// Close lets go of the packs go-git and the reader hold open. Both open them
// again when they next read.
func (s *cachedStorage) Close() error {
	err := s.Storage.Close()
	if s.reader != nil {
		err = errors.Join(err, s.reader.close())
	}
	return err
}
