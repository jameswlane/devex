package deb

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
)

var execCommand = exec.Command

func Install(filePath string, dryRun bool, repo repository.Repository) error {
	log.Info("Starting Install", "filePath", filePath, "dryRun", dryRun)

	// Check if the app is already installed on the system (via dpkg-query)
	log.Info("Checking if .deb package is installed on the system", "filePath", filePath)
	isInstalledOnSystem, err := check_install.IsAppInstalled(filePath)
	if err != nil {
		log.Error("Failed to check if .deb package is installed on system", "filePath", filePath, "error", err)
		return fmt.Errorf("failed to check if .deb package is installed on system: %v", err)
	}

	if isInstalledOnSystem {
		log.Info(fmt.Sprintf(".deb package %s is already installed on the system, skipping installation", filePath))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would run command: sudo dpkg -i %s", filePath))
		log.Info("[Dry Run] Would run command: sudo apt-get install -f -y")
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
		return nil
	}

	// Execute dpkg installation command
	log.Info("Executing dpkg installation command", "command", fmt.Sprintf("sudo dpkg -i %s", filePath))
	cmd := execCommand("sudo", "dpkg", "-i", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to install .deb package", "filePath", filePath, "error", err, "output", string(output))
		return fmt.Errorf("failed to install .deb package: %v - %s", err, string(output))
	}
	log.Info("dpkg installation command executed successfully", "output", string(output))

	// Fix broken dependencies using apt-get
	log.Info("Fixing broken dependencies using apt-get", "command", "sudo apt-get install -f -y")
	cmd = execCommand("sudo", "apt-get", "install", "-f", "-y")
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to fix broken dependencies", "error", err, "output", string(output))
		return fmt.Errorf("failed to fix broken dependencies: %v - %s", err, string(output))
	}
	log.Info("Fixed broken dependencies successfully", "output", string(output))

	// Add to the repository after successful installation
	log.Info("Adding .deb package to repository", "filePath", filePath)
	err = repo.AddApp(filePath)
	if err != nil {
		log.Error("Failed to add .deb package to repository", "filePath", filePath, "error", err)
		return fmt.Errorf("failed to add %s to repository: %v", filePath, err)
	}
	log.Info(".deb package added to repository successfully", "filePath", filePath)

	log.Info("Install completed successfully", "filePath", filePath)
	return nil
}
