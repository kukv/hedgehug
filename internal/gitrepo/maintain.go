package gitrepo

import (
	"errors"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// LooseObjects counts the objects stored one per file under .git/objects.
func (r *Repo) LooseObjects() (int, error) {
	m, err := filepath.Glob(filepath.Join(r.dir, ".git", "objects", "[0-9a-f][0-9a-f]", "*"))
	return len(m), err
}

// Repack writes every loose object into one new pack and then removes the
// loose files, like git repack -d without -a: existing packs are left as they
// are, and no object is dropped, whether a reference reaches it or not. If
// the pack cannot be written, the loose objects stay.
func (r *Repo) Repack() (err error) {
	los, ok := r.r.Storer.(storer.LooseObjectStorer)
	pfw, ok2 := r.r.Storer.(storer.PackfileWriter)
	if !ok || !ok2 {
		return errors.New("the object storage of the repository cannot be repacked")
	}
	var loose []plumbing.Hash
	err = los.ForEachObjectHash(func(h plumbing.Hash) error {
		loose = append(loose, h)
		return nil
	})
	if err != nil || len(loose) == 0 {
		return err
	}
	cfg, err := r.r.Config()
	if err != nil {
		return err
	}
	w, err := pfw.PackfileWriter()
	if err != nil {
		return err
	}
	_, err = packfile.NewEncoder(w, r.r.Storer, false).Encode(loose, cfg.Pack.Window)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	for _, h := range loose {
		if err := los.DeleteLooseObject(h); err != nil {
			return err
		}
	}
	return nil
}
