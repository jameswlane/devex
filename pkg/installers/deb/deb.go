package deb

import (
	"fmt"
	"log"
	"os/exec"
)

// execCommand is a variable to allow mocking for tests
var execCommand = exec.Command

// InstallDeb installs a local .deb file using dpkg and apt-get
func Install(filePath string, dryRun bool) error {
	// Install the .deb package using dpkg
	cmd := execCommand("sudo", "dpkg", "-i", filePath)
	if dryRun {
		log.Printf("[Dry Run] Would run command: %s", cmd.String())
		return nil
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install .deb package: %v - %s", err, string(output))
	}

	// Fix any broken dependencies with apt-get install -f
	cmd = execCommand("sudo", "apt-get", "install", "-f", "-y")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fix broken dependencies: %v - %s", err, string(output))
	}

	return nil
}
