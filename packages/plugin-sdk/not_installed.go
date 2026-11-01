package sdk

import (
	"fmt"
	"strings"
)

// ExitCodeNotInstalled is the exit status of an `is-installed` command when
// the query ran and at least one package is absent. Any other non-zero status
// means the query itself failed, so callers must not treat it as absence.
const ExitCodeNotInstalled = 3

// NotInstalledError reports that an installed-package query succeeded and
// found the listed packages absent. Plugins return it from `is-installed`;
// the host decodes it from the plugin's diagnostics, so callers can tell a
// missing package from a failed query with errors.As.
type NotInstalledError struct {
	Packages []string `json:"packages"`
	Err      error    `json:"-"`
}

func (e *NotInstalledError) Error() string {
	return fmt.Sprintf("not installed: %s", strings.Join(e.Packages, ", "))
}

func (e *NotInstalledError) Unwrap() error { return e.Err }

// ExitCode lets ExitStatus map the error to ExitCodeNotInstalled.
func (e *NotInstalledError) ExitCode() int { return ExitCodeNotInstalled }
