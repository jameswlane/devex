package homebrew

import (
	"os/exec"

)

var log = logger.InitLogger()

// InstallHomebrew installs Homebrew for Linux in non-interactive mode
func InstallHomebrew() error {
	log.LogInfo("Installing Homebrew for Linux (non-interactive)...")
	cmd := exec.Command("sh", "-c", "NONINTERACTIVE=1 /bin/bash -c \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"")
	err := cmd.Run()
	if err != nil {
		log.LogError("Failed to install Homebrew for Linux", err)
		return err
	}
	log.LogInfo("Homebrew for Linux installed successfully (non-interactive)")
	return nil
}
