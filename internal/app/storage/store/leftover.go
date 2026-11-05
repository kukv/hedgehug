package store

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// What a rebuild after a history rewrite (replicate.Rebuild) leaves beside
// the store at dir while it swaps: the rebuilt store, and the old one moved
// away.
const (
	RewriteNewSuffix    = ".rewrite-new"     // <dir>.rewrite-new
	BeforeRewritePrefix = ".before-rewrite-" // <dir>.before-rewrite-<when>
)

// Leftover is what a rebuild left beside the store at dir.
type Leftover struct {
	New    string   // <dir>.rewrite-new, "" when none
	Before []string // <dir>.before-rewrite-*, oldest first
}

// None reports whether nothing was left.
func (l Leftover) None() bool { return l.New == "" && len(l.Before) == 0 }

// FindLeftover looks beside dir for what a rebuild left.
func FindLeftover(dir string) (Leftover, error) {
	entries, err := ReadParent(dir)
	if err != nil {
		return Leftover{}, err
	}
	return LeftoverIn(dir, entries), nil
}

// ReadParent reads the directory dir is in, for LeftoverIn; a caller that
// looks at several stores reads each directory once. A missing directory
// has no entries.
func ReadParent(dir string) ([]fs.DirEntry, error) {
	entries, err := osfs.ReadDir(filepath.Dir(filepath.Clean(dir)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return entries, err
}

// LeftoverIn is FindLeftover over the entries of the directory dir is in
// (ReadParent).
func LeftoverIn(dir string, entries []fs.DirEntry) Leftover {
	var l Leftover
	parent, base := filepath.Split(filepath.Clean(dir))
	if parent == "" {
		parent = "."
	}
	for _, e := range entries { // sorted by name, so the oldest before-rewrite first
		switch name := e.Name(); {
		case name == base+RewriteNewSuffix:
			l.New = filepath.Join(parent, name)
		case strings.HasPrefix(name, base+BeforeRewritePrefix):
			l.Before = append(l.Before, filepath.Join(parent, name))
		}
	}
	return l
}

// LeftoverMessage is what Hedgehug tells of an unfinished rebuild of the
// store called name.
func LeftoverMessage(name string) string {
	return fmt.Sprintf("the rebuild of %s after a history rewrite is unfinished (hedgehug doctor --fix finishes it)", name)
}

// LeftoverError says a store has a rebuild left unfinished.
type LeftoverError struct{ Name string }

func (e *LeftoverError) Error() string { return LeftoverMessage(e.Name) }
