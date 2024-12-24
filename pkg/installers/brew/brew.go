package brew

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
)

var brewExecCommand = exec.Command

func Install(packageName string, dryRun bool, repo repository.Repository) error {
	log.Info("Starting Install", "packageName", packageName, "dryRun", dryRun)

	// Check if the app is already installed on the system
	log.Info("Checking if app is installed on the system", "packageName", packageName)
	isInstalledOnSystem, err := check_install.IsAppInstalled(packageName)
	if err != nil {
		log.Error("Failed to check if app is installed on system", "packageName", packageName, "error", err)
		return fmt.Errorf("failed to check if app is installed on system: %v", err)
	}

	if isInstalledOnSystem {
		log.Info(fmt.Sprintf("%s is already installed on the system, skipping installation", packageName))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would run command: brew install %s", packageName))
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
		return nil
	}

	// Execute the installation command
	log.Info("Executing installation command", "command", fmt.Sprintf("brew install %s", packageName))
	cmd := brewExecCommand("brew", "install", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to install package", "packageName", packageName, "error", err, "output", string(output))
		return fmt.Errorf("failed to install %s: %v - %s", packageName, err, string(output))
	}

	// Add to the repository
	log.Info("Adding app to repository", "packageName", packageName)
	err = repo.AddApp(packageName)
	if err != nil {
		log.Error("Failed to add app to repository", "packageName", packageName, "error", err)
		return fmt.Errorf("failed to add %s to repository: %v", packageName, err)
	}

	log.Info(fmt.Sprintf("%s installed successfully", packageName))
	return nil
}
