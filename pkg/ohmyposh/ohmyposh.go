package ohmyposh

import (
	"fmt"
	"os/exec"

	"github.com/jameswlane/devex/pkg/logger"
)

var log = logger.InitLogger()

// InstallOhMyPosh installs Oh-my-posh and configures it for Zsh
func InstallOhMyPosh() error {
	log.LogInfo("Installing Oh-my-posh...")

	// Install Oh-my-posh using apt-get
	cmd := exec.Command("sudo", "apt-get", "install", "-y", "oh-my-posh")
	if output, err := cmd.CombinedOutput(); err != nil {
		log.LogError("Failed to install Oh-my-posh", err)
		return fmt.Errorf("failed to install Oh-my-posh: %w - %s", err, string(output))
	}

	// Configure Oh-my-posh with Zsh
	configCmd := `echo 'eval "$(oh-my-posh init zsh)"' >> ~/.zshrc`
	cmd = exec.Command("sh", "-c", configCmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.LogError("Failed to configure Oh-my-posh with Zsh", err)
		return fmt.Errorf("failed to configure Oh-my-posh: %w - %s", err, string(output))
	}

	log.LogInfo("Oh-my-posh installed and configured successfully")
	return nil
}
