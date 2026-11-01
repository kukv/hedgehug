//go:build !windows

package sync

import (
	"os"

	"golang.org/x/sys/unix"
)

// redirectStderr makes file descriptor 2 refer to to; what it referred to
// before is let go. closeOld is for Windows.
func redirectStderr(to *os.File, _ bool) error {
	return unix.Dup2(int(to.Fd()), 2) //nolint:gosec // G115: a descriptor fits in an int
}
