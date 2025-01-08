package shell

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// CommandExecutor interface defines a command executor's behavior.
type CommandExecutor interface {
	RunShellCommand(cmd string) (string, error)
}

// AppChecker interface checks if an app is installed.
type AppChecker interface {
	IsAppInstalled(appConfig types.AppConfig) (bool, error)
}

// SwitchToZsh installs Zsh if necessary and switches the user's default shell to Zsh.
func SwitchToZsh(executor CommandExecutor, checker AppChecker) error {
	log.Info("Checking if Zsh is installed")

	// Check if Zsh is installed
	appConfig := types.AppConfig{
		Name:           "Zsh",
		InstallCommand: "zsh",
	}
	isInstalled, err := checker.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Error checking if Zsh is installed", err)
		return fmt.Errorf("error checking if Zsh is installed: %w", err)
	}

	// Install Zsh if it's not installed
	if !isInstalled {
		log.Info("Zsh not installed. Installing Zsh...")
		if _, err := executor.RunShellCommand("sudo apt-get install -y zsh"); err != nil {
			log.Error("Failed to install Zsh", err)
			return fmt.Errorf("failed to install Zsh: %w", err)
		}
		log.Info("Zsh installed successfully")
	}

	// Change default shell to Zsh
	log.Info("Switching default shell to Zsh")
	if _, err := executor.RunShellCommand("chsh -s /bin/zsh"); err != nil {
		log.Error("Failed to switch shell to Zsh", err)
		return fmt.Errorf("failed to switch shell to Zsh: %w", err)
	}

	log.Info("Default shell successfully switched to Zsh")
	return nil
}
