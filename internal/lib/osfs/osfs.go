// Package osfs is where Hedgehug touches the OS: it renames, removes, reads
// and writes files, makes directories, locks files, starts processes and
// compares paths (spec: OS の境界). The rest of the code calls these instead
// of os, os/exec and syscall, which forbidigo enforces, so that what differs
// between OSes stays in the _unix.go files here.
package osfs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// WriteAtomic replaces the file at path with data. It writes a temporary file
// in the same directory, syncs it and renames it over path, so a reader sees
// either the old or the new content, never a part.
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return WriteAtomicInDir(path, data)
}

// WriteAtomicInDir is WriteAtomic for a directory that exists: it creates none,
// and fails with fs.ErrNotExist when the directory is missing.
func WriteAtomicInDir(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// CopyTree copies the directory src to dst, which must not exist: its
// directories, regular files and symbolic links, with their permissions;
// the directories also get the owner's rwx, so that the copy can be written.
// skip names what is left out, by its slash-separated path relative to src;
// nil leaves out nothing.
func CopyTree(src, dst string, skip func(rel string) bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if skip != nil && rel != "." && skip(filepath.ToSlash(rel)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		to := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(to, info.Mode().Perm()|0o700)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, to) //nolint:gosec // G122: src is a store this process holds the lock of, not a tree an attacker can swap
		case d.Type().IsRegular():
			b, err := ReadFile(p)
			if err != nil {
				return err
			}
			return WriteFile(to, b, info.Mode().Perm())
		}
		return nil
	})
}
