package curlpipe

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore"
)

// Install downloads and runs a script from the given URL using curl and pipes it to sh.
func Install(url string, dryRun bool, db *datastore.DB) error {
	// Step 1: Log the action
	log.Info("Running installer via curl pipe", "url", url)

	// Step 2: Prepare the command
	command := fmt.Sprintf("curl -fsSL %s | sh", url)

	// Step 3: Handle dry-run scenario
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would run command: %s", command))
		return nil
	}

	// Step 4: Execute the command
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute curl pipe installer: %v - %s", err, string(output))
	}

	// Step 5: Log success
	log.Info("Installer script executed successfully", "url", url)

	// Step 6: Add to database
	name := extractNameFromURL(url)
	if err := datastore.AddInstalledApp(db, name); err != nil {
		return fmt.Errorf("failed to add %s to database: %v", name, err)
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
