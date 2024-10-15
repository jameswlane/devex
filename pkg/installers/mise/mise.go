package mise

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/logger"
	"os/exec"
	"time"
)

var miseExecCommand = exec.Command

func Install(language string, dryRun bool, db *datastore.DB, logger *logger.Logger) error {
	// Check if the language is already installed
	isInstalledOnSystem, err := check_install.IsAppInstalled(language)
	if err != nil {
		return fmt.Errorf("failed to check if language %s is installed: %v", language, err)
	}

	if isInstalledOnSystem {
		logger.LogInfo(fmt.Sprintf("Language %s is already installed via Mise, skipping installation", language))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		cmd := miseExecCommand("mise", "use", "--global", language)
		logger.LogInfo(fmt.Sprintf("[Dry Run] Would run command: %s", cmd.String()))
		time.Sleep(5 * time.Second)
		return nil
	}

	// Install the language via Mise
	cmd := miseExecCommand("mise", "use", "--global", language)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install language %s via Mise: %v - %s", language, err, string(output))
	}

	// Add the installed language to the database
	err = datastore.AddInstalledApp(db, language)
	if err != nil {
		return fmt.Errorf("failed to add language %s to database: %v", language, err)
	}

	logger.LogInfo(fmt.Sprintf("Language %s installed successfully via Mise", language))
	return nil
}
