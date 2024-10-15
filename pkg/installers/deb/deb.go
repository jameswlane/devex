package deb

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/logger"
	"os/exec"
	"time"
)

var execCommand = exec.Command

func Install(filePath string, dryRun bool, db *datastore.DB, logger *logger.Logger) error {
	// Check if the app is already installed on the system (via dpkg-query)
	isInstalledOnSystem, err := check_install.IsAppInstalled(filePath)
	if err != nil {
		return fmt.Errorf("failed to check if .deb package is installed on system: %v", err)
	}

	if isInstalledOnSystem {
		logger.LogInfo(fmt.Sprintf(".deb package %s is already installed on the system, skipping installation", filePath))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		logger.LogInfo(fmt.Sprintf("[Dry Run] Would run command: sudo dpkg -i %s", filePath))
		logger.LogInfo("[Dry Run] Would run command: sudo apt-get install -f -y")
		time.Sleep(5 * time.Second)
		return nil
	}

	// Execute dpkg installation command
	cmd := execCommand("sudo", "dpkg", "-i", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install .deb package: %v - %s", err, string(output))
	}

	// Fix broken dependencies using apt-get
	cmd = execCommand("sudo", "apt-get", "install", "-f", "-y")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fix broken dependencies: %v - %s", err, string(output))
	}

	// Add to the database after successful installation
	err = datastore.AddInstalledApp(db, filePath)
	if err != nil {
		return fmt.Errorf("failed to add %s to database: %v", filePath, err)
	}

	return nil
}