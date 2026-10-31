//go:build unix

package atomicfile

import "os"

func rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

// ReadFile returns the content of the file at path.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // G304: callers build paths from a store or repository root
}
