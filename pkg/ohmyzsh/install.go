package ohmyzsh

import (
	"os/exec"
)

// InstallOhMyZsh installs Oh-my-zsh
func InstallOhMyZsh() error {
	logger.LogInfo("Installing Oh-my-zsh...")
	cmd := exec.Command("sh", "-c", "$(curl -fsSL https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh)")
	err := cmd.Run()
	if err != nil {
		logger.LogError("Failed to install Oh-my-zsh", "error", err)
		return err
	}
	logger.LogInfo("Oh-my-zsh installed successfully")
	return nil
}
