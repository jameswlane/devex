package shell

import (
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/logger"
	"os/exec"
)

// SwitchToZsh installs Zsh if necessary and switches the user's default shell to Zsh
func SwitchToZsh() error {
	// Check if Zsh is installed
	isInstalled, err := check_install.IsAppInstalled("zsh")
	if err != nil {
		logger.LogError("Error checking if Zsh is installed", err)
		return err
	}

	// Install Zsh if it's not installed
	if !isInstalled {
		logger.LogInfo("Installing Zsh...")
		cmd := exec.Command("sudo", "apt-get", "install", "-y", "zsh")
		if err := cmd.Run(); err != nil {
			logger.LogError("Failed to install Zsh", err)
			return err
		}
		logger.LogInfo("Zsh installed successfully")
	}

	// Change default shell to Zsh
	logger.LogInfo("Switching default shell to Zsh...")
	cmd := exec.Command("chsh", "-s", "/bin/zsh")
	if err := cmd.Run(); err != nil {
		logger.LogError("Failed to switch shell to Zsh", err)
		return err
	}
	logger.LogInfo("Default shell switched to Zsh")

	return nil
}
