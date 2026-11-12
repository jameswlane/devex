package backup

import (
	"fmt"
	"os"
)

// targetExistsError reports that the rename target already exists.
type targetExistsError struct{ from, to string }

func (e *targetExistsError) Error() string {
	return fmt.Sprintf("rename %s %s: %s already exists", e.from, e.to, e.to)
}

func (e *targetExistsError) Unwrap() error { return os.ErrExist }

// renameIfAbsent is the fallback where no exclusive rename is available.
func renameIfAbsent(from, to string) error {
	if pathExists(to) {
		return &targetExistsError{from: from, to: to}
	}
	return os.Rename(from, to)
}
