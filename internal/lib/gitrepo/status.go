package gitrepo

import (
	"sort"

	git "github.com/go-git/go-git/v5"
)

// FileStatus is a file that differs from HEAD or from the index.
type FileStatus struct {
	Path     string // relative to the repository root, slash-separated
	Staged   bool   // the index differs from HEAD
	Unstaged bool   // the work tree differs from the index, or the file is untracked
}

// Status lists the files that are not clean, sorted by path. Ignored files
// are left out.
func (r *Repo) Status() ([]FileStatus, error) {
	w, err := r.r.Worktree()
	if err != nil {
		return nil, err
	}
	st, err := w.Status()
	if err != nil {
		return nil, err
	}
	var out []FileStatus
	for p, s := range st {
		fs := FileStatus{
			Path:     p,
			Staged:   s.Staging != git.Unmodified && s.Staging != git.Untracked,
			Unstaged: s.Worktree != git.Unmodified,
		}
		if fs.Staged || fs.Unstaged {
			out = append(out, fs)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
