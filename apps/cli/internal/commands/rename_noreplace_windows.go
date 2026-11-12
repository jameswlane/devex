//go:build windows

package commands

import (
	"fmt"
	"io/fs"
	"os"
)

// renameNoReplace moves src to dst. Windows never renames a directory onto an
// existing one, so an existing dst is reported as fs.ErrExist.
func renameNoReplace(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		if _, statErr := os.Lstat(dst); statErr == nil {
			return fmt.Errorf("rename %s to %s: %w", src, dst, fs.ErrExist)
		}
		return fmt.Errorf("rename %s to %s: %w", src, dst, err)
	}
	return nil
}
