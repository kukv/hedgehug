// Package atomicfile replaces files whole, so that a reader never sees a
// part of a write.
package atomicfile

import (
	"os"
	"path/filepath"
)

// WriteFile replaces the file at path with data. It writes a temporary file
// in the same directory, syncs it and renames it over path, so a reader sees
// either the old or the new content, never a part.
func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
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
