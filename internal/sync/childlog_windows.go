package sync

import (
	"os"

	"golang.org/x/sys/windows"
)

// redirectStderr makes the standard error handle of the process to's; with
// closeOld the handle it was is closed, so the file behind it is let go. to
// must stay open as long as the handle is the standard one.
func redirectStderr(to *os.File, closeOld bool) error {
	old, _ := windows.GetStdHandle(windows.STD_ERROR_HANDLE)
	if err := windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(to.Fd())); err != nil {
		return err
	}
	if closeOld && old != 0 && old != windows.InvalidHandle {
		return windows.CloseHandle(old)
	}
	return nil
}
