// Package interaction shares command interaction policy without depending on a UI.
package interaction

import (
	"os"

	"github.com/spf13/viper"
)

// IsInteractiveMode preserves guided setup's opt-out and CI detection semantics.
// Viper resolves flags, environment values, and config before CI detection.
func IsInteractiveMode() bool {
	_ = viper.BindEnv("non-interactive", "DEVEX_NONINTERACTIVE", "DEVEX_NON_INTERACTIVE")
	if viper.GetBool("non-interactive") {
		return false
	}
	return os.Getenv("CI") == "" && os.Getenv("GITHUB_ACTIONS") == "" && os.Getenv("GITLAB_CI") == "" && os.Getenv("TERM") != "dumb"
}
