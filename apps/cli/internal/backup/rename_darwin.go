//go:build darwin

package backup

import (
	"errors"

	"golang.org/x/sys/unix"
)

// renameNoReplace renames from to to, failing with os.ErrExist instead of
// replacing anything already at to.
func renameNoReplace(from, to string) error {
	err := unix.RenamexNp(from, to, unix.RENAME_EXCL)
	switch {
	case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EINVAL):
		// The file system lacks RENAME_EXCL.
		return renameIfAbsent(from, to)
	case errors.Is(err, unix.EEXIST):
		return &targetExistsError{from: from, to: to}
	}
	return err
}
