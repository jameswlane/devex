package ohmyposh

import (
	"github.com/jameswlane/devex/pkg/logger"
	"os/exec"
)

var log = logger.InitLogger()

// InstallOhMyPosh installs Oh-my-posh and configures it for Zsh
func InstallOhMyPosh() error {
	log.LogInfo("Installing Oh-my-posh...")
	cmd := exec.Command("sudo", "apt-get", "install", "-y", "oh-my-posh")
	err := cmd.Run()
	if err != nil {
		log.LogError("Failed to install Oh-my-posh", err)
		return err
	}

	// Configure Oh-my-posh with Zsh
	configCmd := `echo 'eval "$(oh-my-posh init zsh)"' >> ~/.zshrc`
	cmd = exec.Command("sh", "-c", configCmd)
	err = cmd.Run()
	if err != nil {
		log.LogError("Failed to configure Oh-my-posh with Zsh", err)
		return err
	}

	log.LogInfo("Oh-my-posh installed and configured successfully")
	return nil
}
