package gitrepo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"

	"github.com/kukv/hedgehug/internal/atomicfile"
)

// LooseObjects counts the objects stored one per file under .git/objects.
func (r *Repo) LooseObjects() (int, error) {
	m, err := filepath.Glob(filepath.Join(r.dir, ".git", "objects", "[0-9a-f][0-9a-f]", "*"))
	return len(m), err
}

// Repack writes every loose object into one new pack and then removes the
// loose files, like git repack -d without -a: existing packs are left as they
// are, and no object is dropped, whether a reference reaches it or not. If
// the pack cannot be written, the loose objects stay and the temporary files
// it made are removed.
func (r *Repo) Repack() error {
	los, ok := r.r.Storer.(storer.LooseObjectStorer)
	if !ok {
		return errors.New("the object storage of the repository cannot be repacked")
	}
	var loose []plumbing.Hash
	err := los.ForEachObjectHash(func(h plumbing.Hash) error {
		loose = append(loose, h)
		return nil
	})
	if err != nil || len(loose) == 0 {
		return err
	}
	if err := r.writePack(loose); err != nil {
		return err
	}
	// The storage reads the list of packs once; make it see the new one
	// before the loose copies go.
	if ri, ok := r.r.Storer.(interface{ Reindex() }); ok {
		ri.Reindex()
	}
	for _, h := range loose {
		if err := los.DeleteLooseObject(h); err != nil {
			return err
		}
	}
	return nil
}

// afterPackWrite runs once writePack has written the temporary pack; tests
// replace it to make a later step fail.
var afterPackWrite = func() error { return nil }

// writePack writes objs as .git/objects/pack/pack-<sum>.pack and its .idx.
// go-git's own pack writer is not used because it can leave its temporary
// file behind on a failure; here every temporary file is named and removed.
// The .idx goes in last, as git does, since it is what makes a pack seen.
func (r *Repo) writePack(objs []plumbing.Hash) (err error) {
	cfg, err := r.r.Config()
	if err != nil {
		return err
	}
	dir := filepath.Join(r.dir, ".git", "objects", "pack")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	var tmps []string
	defer func() {
		if err != nil {
			for _, t := range tmps {
				removeReadOnly(t)
			}
		}
	}()
	pack, err := os.CreateTemp(dir, "tmp_hh_pack_*")
	if err != nil {
		return err
	}
	tmps = append(tmps, pack.Name())
	sum, err := packfile.NewEncoder(pack, r.r.Storer, false).Encode(objs, cfg.Pack.Window)
	if err == nil {
		err = pack.Sync()
	}
	if cerr := pack.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = afterPackWrite()
	}
	if err != nil {
		return err
	}
	idx, err := indexPack(pack.Name())
	if err != nil {
		return err
	}
	ix, err := os.CreateTemp(dir, "tmp_hh_idx_*")
	if err != nil {
		return err
	}
	tmps = append(tmps, ix.Name())
	_, err = idxfile.NewEncoder(ix).Encode(idx)
	if err == nil {
		err = ix.Sync()
	}
	if cerr := ix.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	base := filepath.Join(dir, "pack-"+sum.String())
	if _, serr := os.Stat(base + ".idx"); serr == nil {
		// The same pack is already there; packs are named by their content.
		for _, t := range tmps {
			_ = os.Remove(t)
		}
		return nil
	} else if !errors.Is(serr, fs.ErrNotExist) {
		return serr
	}
	// Read-only for everyone, as git writes them, so that the other users of
	// a shared repository can read them.
	for _, t := range tmps {
		if err := os.Chmod(t, packMode); err != nil {
			return err
		}
	}
	if err := atomicfile.Rename(pack.Name(), base+".pack"); err != nil {
		return err
	}
	if err := atomicfile.Rename(ix.Name(), base+".idx"); err != nil {
		// A .pack without its .idx is never read; it is ours, so remove it.
		removeReadOnly(base + ".pack")
		return err
	}
	return nil
}

const packMode = 0o444

// removeReadOnly removes a file writePack made, which may be read-only by
// then; Windows does not remove a read-only file.
func removeReadOnly(p string) {
	_ = os.Chmod(p, 0o600)
	_ = os.Remove(p)
}

// indexPack reads the pack at path and returns its index.
func indexPack(path string) (*idxfile.MemoryIndex, error) {
	f, err := os.Open(path) //nolint:gosec // G304: a temporary file writePack made
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	w := new(idxfile.Writer)
	p, err := packfile.NewParser(packfile.NewScanner(f), w)
	if err != nil {
		return nil, err
	}
	if _, err := p.Parse(); err != nil {
		return nil, err
	}
	return w.Index()
}
