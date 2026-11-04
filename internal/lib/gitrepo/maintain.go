package gitrepo

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// LooseObjects counts the objects stored one per file under .git/objects.
func (r *Repo) LooseObjects() (int, error) {
	m, err := filepath.Glob(filepath.Join(r.dir, ".git", "objects", "[0-9a-f][0-9a-f]", "*"))
	return len(m), err
}

// PackCap is the size from which a pack is left as it is.
const PackCap = 64 << 20

// packCap is PackCap; tests lower it.
var packCap int64 = PackCap

// smallPack is a pack below packCap, by the path of its .idx.
type smallPack struct {
	idx  string
	size int64
}

// smallPacks lists the packs below packCap, the smallest first.
func (r *Repo) smallPacks() ([]smallPack, error) {
	idxs, err := filepath.Glob(filepath.Join(r.dir, ".git", "objects", "pack", "pack-*.idx"))
	if err != nil {
		return nil, err
	}
	var out []smallPack
	for _, ix := range idxs {
		fi, err := os.Stat(strings.TrimSuffix(ix, ".idx") + ".pack")
		if errors.Is(err, fs.ErrNotExist) {
			continue // being removed
		}
		if err != nil {
			return nil, err
		}
		if fi.Size() < packCap {
			out = append(out, smallPack{ix, fi.Size()})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].size != out[j].size {
			return out[i].size < out[j].size
		}
		return out[i].idx < out[j].idx
	})
	return out, nil
}

// SmallPacks counts the packs smaller than PackCap.
func (r *Repo) SmallPacks() (int, error) {
	ps, err := r.smallPacks()
	return len(ps), err
}

// pickGeometric returns how many of the smallest packs to merge, given their
// sizes in ascending order: one more than the largest i for which
// sizes[i] < 2 * (sizes[0] + ... + sizes[i-1]), or 0 if there is none. What
// is left is geometric: each pack is at least twice the sum of the smaller
// ones, as `git repack --geometric=2` leaves it.
func pickGeometric(sizes []int64) int {
	var sum int64
	n := 0
	for i, s := range sizes {
		if i > 0 && s < 2*sum {
			n = i + 1
		}
		sum += s
	}
	return n
}

// packObjects lists the objects of the pack whose index is at idxPath.
func packObjects(idxPath string) ([]plumbing.Hash, error) {
	f, err := os.Open(idxPath) //nolint:gosec // G304: a pack of this repository
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	idx := idxfile.NewMemoryIndex()
	if err := idxfile.NewDecoder(f).Decode(idx); err != nil {
		return nil, err
	}
	it, err := idx.EntriesByOffset()
	if err != nil {
		return nil, err
	}
	defer func() { _ = it.Close() }()
	var out []plumbing.Hash
	for {
		e, err := it.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, e.Hash)
	}
}

// Consolidate writes the loose objects and the small packs that the
// geometric rule picks into one new pack, then removes what it put in. Packs
// of PackCap or more are left as they are, and no object is dropped,
// whether a reference reaches it or not. What is merged is decided from
// what exists at the start; objects written meanwhile are never touched. If
// the pack cannot be written, everything stays and the temporary files it
// made are removed. An old pack that cannot be removed (Windows, open file)
// is left for a later Consolidate.
func (r *Repo) Consolidate() error {
	los, ok := r.r.Storer.(storer.LooseObjectStorer)
	if !ok {
		return errors.New("the object storage of the repository cannot be consolidated")
	}
	packs, err := r.smallPacks()
	if err != nil {
		return err
	}
	sizes := make([]int64, len(packs))
	for i, p := range packs {
		sizes[i] = p.size
	}
	// One run merges about packCap at most; the pack encoder works out the
	// deltas again, so a bigger merge is slow.
	k := pickGeometric(sizes)
	var total int64
	for _, sz := range sizes[:k] {
		total += sz
	}
	for k > 0 && total > packCap {
		k--
		total -= sizes[k]
	}
	if k < 2 {
		k = 0
	}
	picked := packs[:k]
	var loose []plumbing.Hash
	err = los.ForEachObjectHash(func(h plumbing.Hash) error {
		loose = append(loose, h)
		return nil
	})
	if err != nil || (len(picked) == 0 && len(loose) == 0) {
		return err
	}
	seen := make(map[plumbing.Hash]bool)
	var objs []plumbing.Hash
	add := func(hs []plumbing.Hash) {
		for _, h := range hs {
			if !seen[h] {
				seen[h] = true
				objs = append(objs, h)
			}
		}
	}
	add(loose)
	for _, p := range picked {
		hs, err := packObjects(p.idx)
		if err != nil {
			return err
		}
		add(hs)
	}
	base, err := r.writePack(objs)
	if err != nil {
		return err
	}
	// The storage reads the list of packs once; make it see the new one
	// before the old ones go.
	if ri, ok := r.r.Storer.(interface{ Reindex() }); ok {
		ri.Reindex()
	}
	// The storage keeps the packs it read open; they must not be open when
	// they are removed (Windows). It opens them again when it needs them.
	if err := r.Close(); err != nil {
		return err
	}
	for _, p := range picked {
		old := strings.TrimSuffix(p.idx, ".idx")
		if old == base {
			continue // the new pack is the same as this one
		}
		// The .idx goes first: a .pack alone is never read.
		removeReadOnly(old + ".idx")
		removeReadOnly(old + ".pack")
	}
	// And again now that they are gone, so that the storage does not look
	// for objects in them.
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
func (r *Repo) writePack(objs []plumbing.Hash) (base string, err error) {
	cfg, err := r.r.Config()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(r.dir, ".git", "objects", "pack")
	if err := osfs.MkdirAll(dir, 0o750); err != nil {
		return "", err
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
		return "", err
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
		return "", err
	}
	idx, err := indexPack(pack.Name())
	if err != nil {
		return "", err
	}
	ix, err := os.CreateTemp(dir, "tmp_hh_idx_*")
	if err != nil {
		return "", err
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
		return "", err
	}
	base = filepath.Join(dir, "pack-"+sum.String())
	if _, serr := os.Stat(base + ".idx"); serr == nil {
		// The same pack is already there; packs are named by their content.
		for _, t := range tmps {
			_ = osfs.Remove(t)
		}
		return base, nil
	} else if !errors.Is(serr, fs.ErrNotExist) {
		return "", serr
	}
	// Read-only for everyone, as git writes them, so that the other users of
	// a shared repository can read them.
	for _, t := range tmps {
		if err := os.Chmod(t, packMode); err != nil {
			return "", err
		}
	}
	if err := osfs.Rename(pack.Name(), base+".pack"); err != nil {
		return "", err
	}
	if err := osfs.Rename(ix.Name(), base+".idx"); err != nil {
		// A .pack without its .idx is never read; it is ours, so remove it.
		removeReadOnly(base + ".pack")
		return "", err
	}
	return base, nil
}

const packMode = 0o444

// removeReadOnly removes a file writePack made, which may be read-only by
// then; Windows does not remove a read-only file.
func removeReadOnly(p string) {
	_ = os.Chmod(p, 0o600)
	_ = osfs.Remove(p)
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
