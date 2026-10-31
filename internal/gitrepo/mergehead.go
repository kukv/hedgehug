package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// gitDir is the directory that holds the repository's own files: .git, or
// where a .git file (a linked work tree, a submodule) points.
func (r *Repo) gitDir() (string, error) {
	dot := filepath.Join(r.dir, ".git")
	fi, err := os.Stat(dot)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return dot, nil
	}
	b, err := os.ReadFile(dot) //nolint:gosec // the repository's own .git file
	if err != nil {
		return "", err
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return "", fmt.Errorf("%s: no gitdir line", dot)
	}
	target = strings.TrimSpace(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(r.dir, target)
	}
	return target, nil
}

// MergeHead returns the commit a merge under way merges into HEAD, from
// MERGE_HEAD; ok is false when no merge is under way. Of several heads the
// first is returned.
func (r *Repo) MergeHead() (h plumbing.Hash, ok bool, err error) {
	dir, err := r.gitDir()
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "MERGE_HEAD")) //nolint:gosec // the repository's own file
	if errors.Is(err, fs.ErrNotExist) {
		return plumbing.ZeroHash, false, nil
	}
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	if !plumbing.IsHash(first) {
		return plumbing.ZeroHash, false, fmt.Errorf("MERGE_HEAD: %q is no commit", first)
	}
	return plumbing.NewHash(first), true, nil
}
