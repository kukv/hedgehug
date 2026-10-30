//go:build unix

package store

import "os"

func rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // G304: paths are built from the store root
}
