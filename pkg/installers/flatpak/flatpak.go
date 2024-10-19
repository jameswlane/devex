package flatpak

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/logger"
	"os/exec"
	"time"
)

var flatpakExecCommand = exec.Command

func Install(appID, repo string, dryRun bool, db *datastore.DB, logger *logger.Logger) error {
	// Check if the app is already installed
	isInstalledOnSystem, err := check_install.IsAppInstalled(appID)
	if err != nil {
		return fmt.Errorf("failed to check if Flatpak app is installed: %v", err)
	}

	if isInstalledOnSystem {
		logger.LogInfo(fmt.Sprintf("Flatpak app %s is already installed, skipping installation", appID))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		cmd := flatpakExecCommand("flatpak", "install", repo, appID, "-y")
		logger.LogInfo(fmt.Sprintf("[Dry Run] Would run command: %s", cmd.String()))
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
		return nil
	}

	// Install the Flatpak app
	cmd := flatpakExecCommand("flatpak", "install", repo, appID, "-y")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install Flatpak app %s: %v - %s", appID, err, string(output))
	}

	// Add the installed app to the database
	err = datastore.AddInstalledApp(db, appID)
	if err != nil {
		return fmt.Errorf("failed to add Flatpak app %s to database: %v", appID, err)
	}

	logger.LogInfo(fmt.Sprintf("Flatpak app %s installed successfully", appID))
	return nil
}
