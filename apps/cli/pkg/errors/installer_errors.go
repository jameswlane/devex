package errors

import "fmt"

type InstallerError struct {
	Installer string
	Err       error
}

func (e *InstallerError) Error() string {
	return fmt.Sprintf("installer '%s' failed: %v", e.Installer, e.Err)
}

func (e *InstallerError) Unwrap() error {
	return e.Err
}
