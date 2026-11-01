package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"

	"github.com/kukv/hedgehug/internal/atomicfile"
)

// SetReference replaces the reference file in one rename, so that a reader
// sees the old reference or the new one and never an empty file. go-git
// truncates the file and then writes it; a reader that opens it in between
// finds it empty, which go-git reports as "reference not found" (and
// Repo.Head as "no commit yet").
func (s *cachedStorage) SetReference(ref *plumbing.Reference) error {
	// The same gate go-git's dotgit applies: a name outside refs/ (other than
	// an upper-case pseudo-ref) could address any file under the git directory.
	if !ref.Name().IsSafe() {
		return fmt.Errorf("%w: %q is not under refs/ nor a valid pseudo-ref", dotgit.ErrReferenceNameEscape, string(ref.Name()))
	}
	var content string
	switch ref.Type() {
	case plumbing.SymbolicReference:
		content = fmt.Sprintf("ref: %s\n", ref.Target())
	case plumbing.HashReference:
		content = ref.Hash().String() + "\n"
	default:
		return fmt.Errorf("reference %s has an unknown type", ref.Name())
	}
	path := filepath.Join(s.gitDir, filepath.FromSlash(ref.Name().String()))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- matches go-git (osfs), which git itself reads
		return err
	}
	// The temporary file is outside refs/, where go-git lists references.
	f, err := os.CreateTemp(s.gitDir, "hh-ref-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	// CreateTemp makes the file 0600; go-git writes references as 0666 minus
	// the umask, and other tools expect them readable.
	err = f.Chmod(0o644) // #nosec G302 -- a reference is not a secret
	if err == nil {
		_, err = f.WriteString(content)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = atomicfile.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// CheckAndSetReference sets ref when the reference still is old. hh's own
// writers are serialized by the store lock. The check does not exclude an
// outside git process, and neither does go-git's flock.
func (s *cachedStorage) CheckAndSetReference(ref, old *plumbing.Reference) error {
	if old != nil {
		cur, err := s.Reference(old.Name())
		if err != nil {
			return err
		}
		if cur.Hash() != old.Hash() {
			return storage.ErrReferenceHasChanged
		}
	}
	return s.SetReference(ref)
}
