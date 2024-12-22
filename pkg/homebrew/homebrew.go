package homebrew

import (
	"fmt"
	"os/exec"

	"github.com/jameswlane/devex/pkg/logger"
)

var log = logger.InitLogger()

// InstallHomebrew installs Homebrew for Linux in non-interactive mode
func InstallHomebrew() error {
	log.LogInfo("Installing Homebrew for Linux (non-interactive)...")
	cmd := exec.Command("sh", "-c", "NONINTERACTIVE=1 /bin/bash -c \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.LogError("Failed to install Homebrew for Linux", err)
		return fmt.Errorf("failed to install Homebrew: %w - %s", err, string(output))
	}
	log.LogInfo("Homebrew for Linux installed successfully (non-interactive)")
	return nil
}
