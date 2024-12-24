package curlpipe

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/utils"
)

func Install(url string, dryRun bool, repo repository.Repository) error {
	log.Info("Starting Install", "url", url, "dryRun", dryRun)

	// Prepare the curl command
	command := fmt.Sprintf("curl -fsSL %s | sh", url)
	log.Info("Prepared curl command", "command", command)

	// Execute command as the target user
	log.Info("Executing curl command")
	if err := utils.ExecAsUser(command, dryRun); err != nil {
		log.Error("Failed to execute curl pipe installer", "command", command, "error", err)
		return fmt.Errorf("failed to execute curl pipe installer: %v", err)
	}
	log.Info("Curl command executed successfully")

	// Add to repository
	name := extractNameFromURL(url)
	log.Info("Extracted app name from URL", "name", name)
	if err := repo.AddApp(name); err != nil {
		log.Error("Failed to add app to repository", "name", name, "error", err)
		return fmt.Errorf("failed to add %s to repository: %v", name, err)
	}
	log.Info("App added to repository successfully", "name", name)

	log.Info("Install completed successfully", "url", url)
	return nil
}

// extractNameFromURL generates a simple name for the app based on the URL
func extractNameFromURL(url string) string {
	log.Info("Extracting name from URL", "url", url)
	parts := strings.Split(strings.TrimSpace(url), "/")
	if len(parts) > 0 {
		name := strings.Replace(parts[len(parts)-1], ".run", "", 1)
		log.Info("Extracted name from URL", "name", name)
		return name
	}
	log.Warn("Failed to extract name from URL, returning default name", "url", url)
	return "unknown-app"
}
