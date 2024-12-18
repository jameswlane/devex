package apt

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
)

var (
	aptExecCommand = exec.Command
	aptUpdateRan   = false // Tracks if 'sudo apt update' has already been executed
)

// RunAptUpdate runs 'sudo apt update' if it hasn't already been run
func RunAptUpdate(forceUpdate bool) error {
	if aptUpdateRan && !forceUpdate {
		log.Info("Skipping 'apt update' as it has already been run")
		return nil
	}

	log.Info("Running 'sudo apt update'")
	cmd := aptExecCommand("sudo", "apt-get", "update")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run 'apt update': %v - %s", err, string(output))
	}

	aptUpdateRan = true
	return nil
}

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

	// Step 3: Run 'sudo apt update' if needed
	if err := RunAptUpdate(false); err != nil {
		return err
	}

	// Step 4: Handle dry-run scenario, just log the command
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would run command: sudo apt-get install -y %s", packageName))
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
		return nil // Skip actual execution
	}

	// Step 5: Execute actual installation command
	cmd := aptExecCommand("sudo", "apt-get", "install", "-y", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install %s: %v - %s", packageName, err, string(output))
	}

	// Step 6: After successful installation, add the app to the database
	err = datastore.AddInstalledApp(db, packageName)
	if err != nil {
		return fmt.Errorf("failed to add %s to database: %v", packageName, err)
	}

	// Step 7: Log success
	log.Info(fmt.Sprintf("%s installed successfully and added to the database", packageName))

	return nil
}
