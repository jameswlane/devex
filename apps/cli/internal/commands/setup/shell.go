package setup

import (
	"os"
	"path/filepath"
)

// SupportedShells is the list of shells supported by DevEx.
var SupportedShells = []string{"zsh", "bash", "fish"}

// DetectCurrentShell detects the user's current shell from the $SHELL environment variable,
// falling back to "bash" when $SHELL is empty, unknown, or unsupported.
func DetectCurrentShell() string {
	shellEnv := os.Getenv("SHELL")
	if shellEnv == "" {
		return "bash"
	}

	base := filepath.Base(shellEnv)
	switch base {
	case "zsh", "bash", "fish":
		return base
	default:
		return "bash"
	}
}

// GetDefaultShellIndex returns the index of the detected current shell within the given shells list.
// If the detected shell is not found in the list, it returns the index of "bash", or 0 if neither is found.
func GetDefaultShellIndex(shells []string) int {
	current := DetectCurrentShell()
	for i, s := range shells {
		if s == current {
			return i
		}
	}
	for i, s := range shells {
		if s == "bash" {
			return i
		}
	}
	return 0
}
