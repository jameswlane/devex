package commands

import "time"

// Setup constants for configuration and timeouts
const (
	// Plugin installation timeout - controls how long we wait for plugin installation to complete
	// This timeout is applied to the entire plugin bootstrap process including downloads and setup
	PluginInstallTimeout = 5 * time.Minute

	// Default shell index (zsh) - used when no explicit shell selection is made
	// Corresponds to the index in the shell options array where zsh is the first option
	DefaultShellIndex = 0

	// UI Configuration constants for terminal interface appearance
	ProgressBarWidth = 50  // Width of progress bars in terminal characters (fits in standard 80-column terminals)
	MaxErrorMessages = 100 // Maximum number of error messages to collect to prevent unbounded memory growth during installation failures

	// System resource limits for safe memory management
	MaxPluginsPerPlatform = 20 // Maximum expected plugins per platform, used for slice pre-allocation to optimize performance
)

// FallbackThemes provides default themes when configuration loading fails
// These are the themes available for the 1.0 release and are used when:
// - Configuration files cannot be loaded
// - Network issues prevent theme discovery
// - User is in offline mode during setup
var FallbackThemes = []string{
	"Tokyo Night",  // Popular dark theme with blue/purple accents
	"Synthwave 84", // Retro cyberpunk theme with neon colors
}
