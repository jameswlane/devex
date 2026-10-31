package sdk

import "fmt"

// ManualRemovalRequiredError means no removal was performed. Callers must keep
// installation records and application files until removal succeeds.
type ManualRemovalRequiredError struct {
	Applications []string `json:"applications"`
	Err          error    `json:"-"`
}

func (e *ManualRemovalRequiredError) Error() string {
	return fmt.Sprintf("manual removal required for %v: no uninstall command is configured; see each application's documentation for uninstall instructions, or configure uninstall_command", e.Applications)
}

func (e *ManualRemovalRequiredError) Unwrap() error { return e.Err }
