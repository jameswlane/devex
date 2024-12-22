package pip

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
)

var pipExecCommand = exec.Command

// Install installs a pip package globally, supporting dry-run mode and repository integration
func Install(packageName string, dryRun bool, repo repository.Repository) error {
	// Check if the package is already installed
	isInstalledOnSystem, err := check_install.IsAppInstalled(packageName)
	if err != nil {
		return fmt.Errorf("failed to check if pip package %s is installed: %v", packageName, err)
	}

	if isInstalledOnSystem {
		log.Info(fmt.Sprintf("Pip package %s is already installed, skipping installation", packageName))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		cmd := pipExecCommand("pip", "install", packageName)
		log.Info(fmt.Sprintf("[Dry Run] Would run command: %s", cmd.String()))
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
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

	// Add the installed package to the repository
	err = repo.AddApp(packageName)
	if err != nil {
		return fmt.Errorf("failed to add pip package %s to repository: %v", packageName, err)
	}

	log.Info(fmt.Sprintf("Pip package %s installed successfully", packageName))
	return nil
}
