package check_install

import (
	"os/exec"
)

// checkInstallExecCommand is a variable to allow mocking for tests
var checkInstallExecCommand = exec.Command

// IsAppInstalled checks if a given application is installed by using 'command -v'
func IsAppInstalled(appName string) (bool, error) {
	cmd := checkInstallExecCommand("command", "-v", appName)
	err := cmd.Run()
	if err != nil {
		return false, nil // The app is not installed
	}
	return true, nil // The app is installed
}
