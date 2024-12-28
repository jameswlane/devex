package shell

import (
	"os/exec"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/logger"
	"github.com/jameswlane/devex/pkg/types"
)

var log = logger.InitLogger()

// SwitchToZsh installs Zsh if necessary and switches the user's default shell to Zsh
func SwitchToZsh() error {
	// Check if Zsh is installed
	appConfig := types.AppConfig{
		Name:           "Zsh",
		InstallCommand: "zsh",
	}
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.LogError("Error checking if Zsh is installed", err)
		return err
	}

	// Install Zsh if it's not installed
	if !isInstalled {
		log.LogInfo("Installing Zsh...")
		cmd := exec.Command("sudo", "apt-get", "install", "-y", "zsh")
		if err := cmd.Run(); err != nil {
			log.LogError("Failed to install Zsh", err)
			return err
		}
		log.LogInfo("Zsh installed successfully")
	}

	// Change default shell to Zsh
	log.LogInfo("Switching default shell to Zsh...")
	cmd := exec.Command("chsh", "-s", "/bin/zsh")
	if err := cmd.Run(); err != nil {
		log.LogError("Failed to switch shell to Zsh", err)
		return err
	}
	log.LogInfo("Default shell switched to Zsh")

	return nil
}
