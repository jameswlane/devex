//go:build !linux && !darwin

package backup

// renameNoReplace renames from to to, failing with os.ErrExist instead of
// replacing anything already at to. Without an exclusive rename the check
// is best effort; renaming a directory onto an existing one fails on these
// platforms unless it is empty.
func renameNoReplace(from, to string) error { return renameIfAbsent(from, to) }
