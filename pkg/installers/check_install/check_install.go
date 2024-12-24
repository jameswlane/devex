package check_install

import (
	"os/exec"

	"github.com/charmbracelet/log"
)

// checkInstallExecCommand is a variable to allow mocking for tests
var checkInstallExecCommand = exec.Command

// IsAppInstalled checks if a given application is installed by using 'command -v'
func IsAppInstalled(appName string) (bool, error) {
	log.Info("Checking if app is installed", "appName", appName)
	cmd := checkInstallExecCommand("command", "-v", appName)
	err := cmd.Run()
	if err != nil {
		log.Warn("App is not installed", "appName", appName, "error", err)
		return false, nil // The app is not installed
	}
	log.Info("App is installed", "appName", appName)
	return true, nil // The app is installed
}
