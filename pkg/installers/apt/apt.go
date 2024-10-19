package apt

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"os/exec"
	"time"
)

var aptExecCommand = exec.Command

func Install(packageName string, dryRun bool, db *datastore.DB) error {
	// Step 1: Check if the app is installed on the system
	isInstalledOnSystem, err := check_install.IsAppInstalled(packageName)
	if err != nil {
		return fmt.Errorf("failed to check if app is installed on system: %v", err)
	}

	// Step 2: If already installed, log and skip the installation
	if isInstalledOnSystem {
		log.Info(fmt.Sprintf("%s is already installed on the system, skipping installation", packageName))
		return nil
	}

	// Step 3: Handle dry-run scenario, just log the command
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would run command: sudo apt-get install -y %s", packageName))
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
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
	log.Info(fmt.Sprintf("%s installed successfully and added to the database", packageName))

	return nil
}
