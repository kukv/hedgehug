package gitrepo

import (
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// cachedStorage looks an object up in memory before it touches the disk.
// go-git opens the object file, or the packfile, before it looks in its
// cache, and on Windows each open and each stat of the path costs tens of
// microseconds. An object never changes under its hash, so the copy in
// memory is the object whether or not the file still exists.
type cachedStorage struct {
	*filesystem.Storage
	objects cache.Object
}

func newCachedStorage(s *filesystem.Storage) *cachedStorage {
	return &cachedStorage{Storage: s, objects: cache.NewObjectLRUDefault()}
}

func (s *cachedStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	if o, ok := s.objects.Get(h); ok && (t == plumbing.AnyObject || o.Type() == t) {
		return o, nil
	}
	o, err := s.Storage.EncodedObject(t, h)
	if err != nil {
		return nil, err
	}
	s.objects.Put(o)
	return o, nil
}
