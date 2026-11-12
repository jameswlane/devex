//go:build !linux && !windows

package commands

// renameNoReplace moves src to dst and fails with fs.ErrExist when dst exists.
func renameNoReplace(src, dst string) error {
	return reserveAndRename(src, dst)
}
