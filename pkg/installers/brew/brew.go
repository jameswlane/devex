package brew

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/logger"
	"os/exec"
	"time"
)

var brewExecCommand = exec.Command

func Install(packageName string, dryRun bool, db *datastore.DB, logger *logger.Logger) error {
	// Check if the app is already installed on the system
	isInstalledOnSystem, err := check_install.IsAppInstalled(packageName)
	if err != nil {
		return fmt.Errorf("failed to check if app is installed on system: %v", err)
	}

	if isInstalledOnSystem {
		logger.LogInfo(fmt.Sprintf("%s is already installed on the system, skipping installation", packageName))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		logger.LogInfo(fmt.Sprintf("[Dry Run] Would run command: brew install %s", packageName))
		time.Sleep(5 * time.Second)
		return nil
	}

	// Execute the installation command
	cmd := brewExecCommand("brew", "install", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install %s: %v - %s", packageName, err, string(output))
	}

	// Add to the database
	err = datastore.AddInstalledApp(db, packageName)
	if err != nil {
		return fmt.Errorf("failed to add %s to database: %v", packageName, err)
	}

	return nil
}
