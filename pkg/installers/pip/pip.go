package pip

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/logger"
	"os"
	"os/exec"
	"time"
)

var pipExecCommand = exec.Command

// Install installs a pip package globally, supporting dry-run mode and datastore integration
func Install(packageName string, dryRun bool, db *datastore.DB, logger *logger.Logger) error {
	// Check if the package is already installed
	isInstalledOnSystem, err := check_install.IsAppInstalled(packageName)
	if err != nil {
		return fmt.Errorf("failed to check if pip package %s is installed: %v", packageName, err)
	}

	if isInstalledOnSystem {
		logger.LogInfo(fmt.Sprintf("Pip package %s is already installed, skipping installation", packageName))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		cmd := pipExecCommand("pip", "install", packageName)
		logger.LogInfo(fmt.Sprintf("[Dry Run] Would run command: %s", cmd.String()))
		time.Sleep(5 * time.Second)
		return nil
	}

	// Install the pip package
	cmd := pipExecCommand("pip", "install", packageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install pip package %s: %v - %s", packageName, err, string(output))
	}

	// Add the installed package to the database
	err = datastore.AddInstalledApp(db, packageName)
	if err != nil {
		return fmt.Errorf("failed to add pip package %s to database: %v", packageName, err)
	}

	logger.LogInfo(fmt.Sprintf("Pip package %s installed successfully", packageName))
	return nil
}
