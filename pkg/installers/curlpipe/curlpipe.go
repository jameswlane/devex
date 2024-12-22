package curlpipe

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/utils"
)

func Install(url string, dryRun bool, repo repository.Repository) error {
	log.Info("Running installer via curl pipe", "url", url)

	// Prepare the curl command
	command := fmt.Sprintf("curl -fsSL %s | sh", url)

	// Execute command as the target user
	if err := utils.ExecAsUser(command, dryRun); err != nil {
		return fmt.Errorf("failed to execute curl pipe installer: %v", err)
	}

	// Add to repository
	name := extractNameFromURL(url)
	if err := repo.AddApp(name); err != nil {
		return fmt.Errorf("failed to add %s to repository: %v", name, err)
	}

	return nil
}

// extractNameFromURL generates a simple name for the app based on the URL
func extractNameFromURL(url string) string {
	parts := strings.Split(strings.TrimSpace(url), "/")
	if len(parts) > 0 {
		return strings.Replace(parts[len(parts)-1], ".run", "", 1)
	}
	return "unknown-app"
}
