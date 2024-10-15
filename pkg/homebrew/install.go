package homebrew

import (
	"os/exec"
)

// InstallHomebrew installs Homebrew for Linux in non-interactive mode
func InstallHomebrew() error {
	logger.LogInfo("Installing Homebrew for Linux (non-interactive)...")
	cmd := exec.Command("sh", "-c", "NONINTERACTIVE=1 /bin/bash -c \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"")
	err := cmd.Run()
	if err != nil {
		logger.LogError("Failed to install Homebrew for Linux", "error", err)
		return err
	}
	logger.LogInfo("Homebrew for Linux installed successfully (non-interactive)")
	return nil
}
