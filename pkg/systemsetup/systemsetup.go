package systemsetup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// UpdateApt updates the apt package list and installs necessary packages
func UpdateApt() error {
	fmt.Println("Updating apt and installing necessary packages...")
	cmd := exec.Command("sudo", "apt", "update", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update apt: %v", err)
	}

	cmd = exec.Command("sudo", "apt", "install", "-y", "curl", "git", "unzip")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install required packages: %v", err)
	}

	fmt.Println("Apt update and package installation complete.")
	return nil
}

// DisableSleepSettings ensures the computer doesn't go to sleep during installation
func DisableSleepSettings() error {
	fmt.Println("Disabling sleep settings...")

	// Disable screen lock
	cmd := exec.Command("gsettings", "set", "org.gnome.desktop.screensaver", "lock-enabled", "false")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable screen lock: %v", err)
	}

	// Set idle delay to 0 (no sleep)
	cmd = exec.Command("gsettings", "set", "org.gnome.desktop.session", "idle-delay", "0")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable idle sleep: %v", err)
	}

	fmt.Println("Sleep settings disabled.")
	return nil
}

// RunInstallers executes all installer scripts
func RunInstallers(installersDir string) error {
	fmt.Println("Running installer scripts...")

	installers, err := filepath.Glob(filepath.Join(installersDir, "*.sh"))
	if err != nil {
		return fmt.Errorf("failed to find installer scripts: %v", err)
	}

	for _, script := range installers {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("source %s", script))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run installer script %s: %v", script, err)
		}
	}

	fmt.Println("All installer scripts executed successfully.")
	return nil
}

// UpgradeSystem upgrades all installed packages
func UpgradeSystem() error {
	fmt.Println("Upgrading system packages...")
	cmd := exec.Command("sudo", "apt", "upgrade", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upgrade system packages: %v", err)
	}
	fmt.Println("System upgrade complete.")
	return nil
}

// RevertSleepSettings reverts sleep and lock settings to normal
func RevertSleepSettings() error {
	fmt.Println("Reverting sleep settings...")

	// Enable screen lock
	cmd := exec.Command("gsettings", "set", "org.gnome.desktop.screensaver", "lock-enabled", "true")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable screen lock: %v", err)
	}

	// Set idle delay to 300 seconds (5 minutes)
	cmd = exec.Command("gsettings", "set", "org.gnome.desktop.session", "idle-delay", "300")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to revert idle delay: %v", err)
	}

	fmt.Println("Sleep settings reverted to normal.")
	return nil
}

// Logout logs the user out to apply changes
func Logout() error {
	fmt.Println("Logging out to apply changes...")

	cmd := exec.Command("gnome-session-quit", "--logout", "--no-prompt")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to log out: %v", err)
	}

	return nil
}
