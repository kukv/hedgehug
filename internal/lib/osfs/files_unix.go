//go:build unix

package osfs

import "os"

// Rename renames oldpath to newpath, replacing it.
func Rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

// Remove removes the file or the empty directory at path.
func Remove(path string) error { return os.Remove(path) }

// RemoveAll removes path and everything it holds. A missing path is no error.
func RemoveAll(path string) error { return os.RemoveAll(path) }

// ReadFile returns the content of the file at path.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // G304: callers build paths from a store or repository root
}

// WriteFile writes data to the file at path, made with perm when missing.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// OpenFile opens the file at path with flag, made with perm when flag has
// os.O_CREATE.
func OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, perm) //nolint:gosec // G304: callers build paths from a store or repository root
}

// Open opens the file at path for reading.
func Open(path string) (*os.File, error) {
	return os.Open(path) //nolint:gosec // G304: callers build paths from a store or repository root
}

// CreateTemp makes a new file in dir, named by pattern with a random part in
// place of its last *, and opens it for reading and writing.
func CreateTemp(dir, pattern string) (*os.File, error) { return os.CreateTemp(dir, pattern) }

// MkdirTemp makes a new directory in dir, named as CreateTemp names a file.
func MkdirTemp(dir, pattern string) (string, error) { return os.MkdirTemp(dir, pattern) }

// Mkdir makes the directory path, with perm. Its parent must exist.
func Mkdir(path string, perm os.FileMode) error { return os.Mkdir(path, perm) }

// MkdirAll makes the directory path and the parents it lacks, with perm.
func MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

// Stat describes the file at path, following a symbolic link.
func Stat(path string) (os.FileInfo, error) { return os.Stat(path) }

// Lstat describes the file at path; a symbolic link is described itself.
func Lstat(path string) (os.FileInfo, error) { return os.Lstat(path) }

// ReadDir lists the directory path, sorted by name.
func ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }

// Chmod sets the mode of the file at path.
func Chmod(path string, mode os.FileMode) error { return os.Chmod(path, mode) }

// SamePath reports whether a and b name the same file. On unix that is the
// same string, as the callers have always compared them.
func SamePath(a, b string) bool { return a == b }
