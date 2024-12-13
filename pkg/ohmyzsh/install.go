package ohmyzsh

import (
	"os/exec"

)

var log = logger.InitLogger()

// InstallOhMyZsh installs Oh-my-zsh
func InstallOhMyZsh() error {
	log.LogInfo("Installing Oh-my-zsh...")
	cmd := exec.Command("sh", "-c", "$(curl -fsSL https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh)")
	err := cmd.Run()
	if err != nil {
		log.LogError("Failed to install Oh-my-zsh", err)
		return err
	}
	log.LogInfo("Oh-my-zsh installed successfully")
	return nil
}
