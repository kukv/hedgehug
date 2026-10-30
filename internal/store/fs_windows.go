//go:build windows

package store

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

// rename retries when another program, such as a virus scanner, has the
// file open for a moment.
func rename(oldpath, newpath string) error {
	return retryRename(os.Rename, transientRenameError, oldpath, newpath)
}

func transientRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// readFile opens path with FILE_SHARE_DELETE, so reading a file does not
// stop another writer from renaming over it.
func readFile(path string) ([]byte, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	// The handle is read-only, so a Close error cannot lose data.
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}
