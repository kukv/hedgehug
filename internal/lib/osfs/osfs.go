// Package osfs is where Hedgehug touches the OS: it renames, removes, reads
// and writes files, makes directories, locks files, starts processes and
// compares paths (spec: OS の境界). The rest of the code calls these instead
// of os, os/exec and syscall, which forbidigo enforces, so that what differs
// between OSes stays in the _unix.go files here.
package osfs

import (
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
