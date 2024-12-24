package flatpak

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/utils"
)

func Install(appID, repoURL string, dryRun bool, repo repository.Repository) error {
	log.Info("Starting Install", "appID", appID, "repoURL", repoURL, "dryRun", dryRun)

	// Check if the app is already installed
	log.Info("Checking if Flatpak app is installed", "appID", appID)
	isInstalled, err := check_install.IsAppInstalled(appID)
	if err != nil {
		log.Error("Failed to check if Flatpak app is installed", "appID", appID, "error", err)
		return fmt.Errorf("failed to check if Flatpak app is installed: %v", err)
	}

	if isInstalled {
		log.Info(fmt.Sprintf("Flatpak app %s is already installed, skipping installation", appID))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would add repo: %s", repoURL))
		log.Info(fmt.Sprintf("[Dry Run] Would install app: %s", appID))
		return nil
	}

	// Add the Flatpak repository
	if repoURL != "" {
		log.Info("Adding Flatpak repository", "repoURL", repoURL)
		command := fmt.Sprintf("flatpak remote-add --if-not-exists %s", repoURL)
		if err := utils.ExecAsUser(command, dryRun); err != nil {
			log.Error("Failed to add Flatpak repository", "repoURL", repoURL, "error", err)
			return fmt.Errorf("failed to add Flatpak repository: %v", err)
		}
		log.Info("Flatpak repository added successfully", "repoURL", repoURL)
	}

	// Install the Flatpak app
	log.Info("Installing Flatpak app", "appID", appID)
	command := fmt.Sprintf("flatpak install -y %s", appID)
	if err := utils.ExecAsUser(command, dryRun); err != nil {
		log.Error("Failed to install Flatpak app", "appID", appID, "error", err)
		return fmt.Errorf("failed to install Flatpak app: %v", err)
	}
	log.Info("Flatpak app installed successfully", "appID", appID)

	// Add the installed app to the repository
	log.Info("Adding Flatpak app to repository", "appID", appID)
	if err := repo.AddApp(appID); err != nil {
		log.Error("Failed to add Flatpak app to repository", "appID", appID, "error", err)
		return fmt.Errorf("failed to add Flatpak app %s to repository: %v", appID, err)
	}
	log.Info("Flatpak app added to repository successfully", "appID", appID)

	log.Info("Install completed successfully", "appID", appID)
	return nil
}
