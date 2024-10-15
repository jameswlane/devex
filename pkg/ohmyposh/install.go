package ohmyposh

import (
	"os/exec"
)

// InstallOhMyPosh installs Oh-my-posh and configures it for Zsh
func InstallOhMyPosh() error {
	logger.LogInfo("Installing Oh-my-posh...")
	cmd := exec.Command("sudo", "apt-get", "install", "-y", "oh-my-posh")
	err := cmd.Run()
	if err != nil {
		logger.LogError("Failed to install Oh-my-posh", "error", err)
		return err
	}

	// Configure Oh-my-posh with Zsh
	configCmd := `echo 'eval "$(oh-my-posh init zsh)"' >> ~/.zshrc`
	cmd = exec.Command("sh", "-c", configCmd)
	err = cmd.Run()
	if err != nil {
		logger.LogError("Failed to configure Oh-my-posh with Zsh", "error", err)
		return err
	}

	logger.LogInfo("Oh-my-posh installed and configured successfully")
	return nil
}
