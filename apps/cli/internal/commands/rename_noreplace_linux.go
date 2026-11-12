//go:build linux

package commands

import (
	"errors"
	"fmt"
	"io/fs"

	"golang.org/x/sys/unix"
)

// renameNoReplace moves src to dst and fails with fs.ErrExist when dst exists,
// even as an empty directory, which a plain rename would replace.
func renameNoReplace(src, dst string) error {
	err := unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_NOREPLACE)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.EEXIST):
		return fmt.Errorf("rename %s to %s: %w", src, dst, fs.ErrExist)
	case errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EINVAL):
		// The kernel or file system lacks RENAME_NOREPLACE.
		return reserveAndRename(src, dst)
	default:
		return fmt.Errorf("rename %s to %s: %w", src, dst, err)
	}
}
