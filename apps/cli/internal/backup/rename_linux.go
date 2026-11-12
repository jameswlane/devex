//go:build linux

package backup

import (
	"errors"

	"golang.org/x/sys/unix"
)

// renameNoReplace renames from to to, failing with os.ErrExist instead of
// replacing anything already at to.
func renameNoReplace(from, to string) error {
	err := unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
	switch {
	case errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EINVAL):
		// The kernel or file system lacks RENAME_NOREPLACE.
		return renameIfAbsent(from, to)
	case errors.Is(err, unix.EEXIST):
		return &targetExistsError{from: from, to: to}
	}
	return err
}
