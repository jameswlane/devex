//go:build !windows

package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// reserveAndRename claims dst with an exclusive mkdir and renames src onto the
// claimed, empty directory with rename(2), which replaces only an empty
// directory (os.Rename refuses any existing one). A rename onto a directory
// that has gained entries fails, so nothing written there by another command
// is replaced. Only the reservation is removed on failure, and only while it
// is still empty.
func reserveAndRename(src, dst string) error {
	if err := os.Mkdir(dst, 0o750); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("rename %s to %s: %w", src, dst, fs.ErrExist)
		}
		return fmt.Errorf("reserve %s: %w", dst, err)
	}
	if err := syscall.Rename(src, dst); err != nil {
		if os.Remove(dst) != nil {
			// Not empty: another command wrote into it, so it is theirs now.
			return fmt.Errorf("rename %s to %s: %w", src, dst, fs.ErrExist)
		}
		return fmt.Errorf("rename %s to %s: %w", src, dst, err)
	}
	return nil
}
