package mise

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
)

var miseExecCommand = exec.Command

func Install(language string, dryRun bool, repo repository.Repository) error {
	log.Info("Starting Install", "language", language, "dryRun", dryRun)

	// Check if the language is already installed
	log.Info("Checking if language is installed", "language", language)
	isInstalledOnSystem, err := check_install.IsAppInstalled(language)
	if err != nil {
		log.Error("Failed to check if language is installed", "language", language, "error", err)
		return fmt.Errorf("failed to check if language %s is installed: %v", language, err)
	}

	if isInstalledOnSystem {
		log.Info(fmt.Sprintf("Language %s is already installed via Mise, skipping installation", language))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		cmd := miseExecCommand("mise", "use", "--global", language)
		log.Info(fmt.Sprintf("[Dry Run] Would run command: %s", cmd.String()))
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
		return nil
	}

	// Install the language via Mise
	log.Info("Installing language via Mise", "language", language)
	cmd := miseExecCommand("mise", "use", "--global", language)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to install language via Mise", "language", language, "error", err, "output", string(output))
		return fmt.Errorf("failed to install language %s via Mise: %v - %s", language, err, string(output))
	}
	log.Info("Language installed via Mise successfully", "language", language, "output", string(output))

	// Add the installed language to the repository
	log.Info("Adding language to repository", "language", language)
	err = repo.AddApp(language)
	if err != nil {
		log.Error("Failed to add language to repository", "language", language, "error", err)
		return fmt.Errorf("failed to add language %s to repository: %v", language, err)
	}
	log.Info("Language added to repository successfully", "language", language)

	log.Info("Install completed successfully", "language", language)
	return nil
}
