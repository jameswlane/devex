//go:build windows

package security

import (
	"os/exec"
)

// setPlatformSpecificAttrs sets Windows-specific process attributes
func setPlatformSpecificAttrs(cmd *exec.Cmd) {
	// Windows doesn't support process groups in the same way as Unix
	// No additional attributes needed for Windows
}
