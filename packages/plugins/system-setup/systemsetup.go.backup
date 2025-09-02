package main

import (
	"fmt"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/utils"
)

// UpdateApt updates the apt package list and installs necessary packages.
func UpdateApt() error {
	log.Info("Updating apt package list and installing necessary packages")

	// Update apt package list
	if _, err := utils.CommandExec.RunShellCommand("sudo apt update -y"); err != nil {
		log.Error("Failed to update apt", err)
		return fmt.Errorf("failed to update apt: %w", err)
	}

	// Install required packages
	if _, err := utils.CommandExec.RunShellCommand("sudo apt install -y curl git unzip"); err != nil {
		log.Error("Failed to install required packages", err)
		return fmt.Errorf("failed to install required packages: %w", err)
	}

	log.Info("Apt update and package installation complete")
	return nil
}

// UpgradeSystem upgrades all installed packages.
func UpgradeSystem() error {
	log.Info("Upgrading system packages")

	// Upgrade packages
	if _, err := utils.CommandExec.RunShellCommand("sudo apt upgrade -y"); err != nil {
		log.Error("Failed to upgrade system packages", err)
		return fmt.Errorf("failed to upgrade system packages: %w", err)
	}

	log.Info("System upgrade complete")
	return nil
}

// DisableSleepSettings ensures the computer doesn't go to sleep during installation.
func DisableSleepSettings() error {
	log.Info("Disabling sleep settings")

	// Disable screen lock
	if _, err := utils.CommandExec.RunShellCommand("gsettings set org.gnome.desktop.screensaver lock-enabled false"); err != nil {
		log.Error("Failed to disable screen lock", err)
		return fmt.Errorf("failed to disable screen lock: %w", err)
	}

	// Set idle delay to 0 (no sleep)
	if _, err := utils.CommandExec.RunShellCommand("gsettings set org.gnome.desktop.session idle-delay 0"); err != nil {
		log.Error("Failed to disable idle sleep", err)
		return fmt.Errorf("failed to disable idle sleep: %w", err)
	}

	log.Info("Sleep settings disabled")
	return nil
}

// RevertSleepSettings reverts sleep and lock settings to normal.
func RevertSleepSettings() error {
	log.Info("Reverting sleep settings to default")

	// Enable screen lock
	if _, err := utils.CommandExec.RunShellCommand("gsettings set org.gnome.desktop.screensaver lock-enabled true"); err != nil {
		log.Error("Failed to enable screen lock", err)
		return fmt.Errorf("failed to enable screen lock: %w", err)
	}

	// Set idle delay to 300 seconds (5 minutes)
	if _, err := utils.CommandExec.RunShellCommand("gsettings set org.gnome.desktop.session idle-delay 300"); err != nil {
		log.Error("Failed to revert idle delay", err)
		return fmt.Errorf("failed to revert idle delay: %w", err)
	}

	log.Info("Sleep settings reverted to normal")
	return nil
}

// Logout logs the user out to apply changes.
func Logout() error {
	log.Info("Logging out to apply changes")

	// Execute logout command
	if _, err := utils.CommandExec.RunShellCommand("gnome-session-quit --logout --no-prompt"); err != nil {
		log.Error("Failed to log out", err)
		return fmt.Errorf("failed to log out: %w", err)
	}

	log.Info("Logout successful")
	return nil
}
