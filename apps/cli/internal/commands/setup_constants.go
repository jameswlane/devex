package commands

import "time"

// Setup constants for configuration and timeouts
const (
	// Plugin installation timeout
	PluginInstallTimeout = 5 * time.Minute

	// Default shell index (zsh)
	DefaultShellIndex = 0
)

// FallbackThemes provides default themes when configuration loading fails
// These are the themes available for the 1.0 release
var FallbackThemes = []string{
	"Tokyo Night",
	"Synthwave 84",
}
