package gitrepo

import (
	"path/filepath"

	git "github.com/go-git/go-git/v5"
)

// LooseObjects counts the objects stored one per file under .git/objects.
func (r *Repo) LooseObjects() (int, error) {
	m, err := filepath.Glob(filepath.Join(r.dir, ".git", "objects", "[0-9a-f][0-9a-f]", "*"))
	return len(m), err
}

// Repack packs every object reachable from a reference into one pack and
// removes the loose objects and the old packs it replaces. An object that no
// reference reaches is dropped if it was in an old pack and kept if it was
// loose.
func (r *Repo) Repack() error {
	return r.r.RepackObjects(&git.RepackConfig{})
}
