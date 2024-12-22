package flatpak

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/utils"
)

func Install(appID, repoURL string, dryRun bool, repo repository.Repository) error {
	// Check if the app is already installed
	isInstalled, err := check_install.IsAppInstalled(appID)
	if err != nil {
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
		command := fmt.Sprintf("flatpak remote-add --if-not-exists %s", repoURL)
		if err := utils.ExecAsUser(command, dryRun); err != nil {
			return fmt.Errorf("failed to add Flatpak repository: %v", err)
		}
	}

	// Add the installed app to the repository
	if err := repo.AddApp(appID); err != nil {
		return fmt.Errorf("failed to add Flatpak app %s to repository: %v", appID, err)
	}

	log.Info(fmt.Sprintf("Flatpak app %s installed successfully", appID))
	return nil
}
