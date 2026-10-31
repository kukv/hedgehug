// Package atomicfile replaces files whole, so that a reader never sees a
// part of a write, and reads them without stopping such a replacement.
package atomicfile

import (
	"os"
	"path/filepath"
	"time"
)

// renameRetries is how many times a rename is tried again after a transient
// failure, renameInterval apart.
const renameRetries = 10

var renameInterval = 50 * time.Millisecond

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
		err = rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// retryRename calls rename until it succeeds, fails with an error retryable
// does not accept, or has been tried again renameRetries times.
func retryRename(rename func(string, string) error, retryable func(error) bool, oldpath, newpath string) error {
	err := rename(oldpath, newpath)
	for i := 0; i < renameRetries && err != nil && retryable(err); i++ {
		time.Sleep(renameInterval)
		err = rename(oldpath, newpath)
	}
	return err
}
