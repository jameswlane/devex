package apt

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/logger"
	"os/exec"
)

var aptExecCommand = exec.Command

func Install(packageName string, dryRun bool, db *datastore.DB, logger *logger.Logger) error {
	// Step 1: Check if the app is installed on the system
	isInstalledOnSystem, err := check_install.IsAppInstalled(packageName)
	if err != nil {
		return fmt.Errorf("failed to check if app is installed on system: %v", err)
	}

	// Step 2: If already installed, log and skip the installation
	if isInstalledOnSystem {
		logger.LogInfo(fmt.Sprintf("%s is already installed on the system, skipping installation", packageName))
		return nil
	}

	// Step 3: Handle dry-run scenario, just log the command
	if dryRun {
		logger.LogInfo(fmt.Sprintf("[Dry Run] Would run command: sudo apt-get install -y %s", packageName))
		return nil // Skip actual execution
	}

	// Step 4: Execute actual installation command
	cmd := aptExecCommand("sudo", "apt-get", "install", "-y", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install %s: %v - %s", packageName, err, string(output))
	}

	// Step 5: After successful installation, add the app to the database
	err = datastore.AddInstalledApp(db, packageName)
	if err != nil {
		return fmt.Errorf("failed to add %s to database: %v", packageName, err)
	}

	// Step 6: Log success
	logger.LogInfo(fmt.Sprintf("%s installed successfully and added to the database", packageName))

	return nil
}
