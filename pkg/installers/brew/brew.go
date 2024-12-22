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
	// Check if the app is already installed on the system
	isInstalledOnSystem, err := check_install.IsAppInstalled(packageName)
	if err != nil {
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
	cmd := brewExecCommand("brew", "install", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install %s: %v - %s", packageName, err, string(output))
	}

	// Add to the repository
	err = repo.AddApp(packageName)
	if err != nil {
		return fmt.Errorf("failed to add %s to repository: %v", packageName, err)
	}

	return nil
}
