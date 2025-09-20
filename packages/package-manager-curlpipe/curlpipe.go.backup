package curlpipe

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type CurlPipeInstaller struct{}

func New() *CurlPipeInstaller {
	return &CurlPipeInstaller{}
}

func (c *CurlPipeInstaller) Install(command string, repo types.Repository) error {
	log.Debug("CurlPipe Installer: Starting installation", "command", command)

	// Execute the curl | sh command
	_, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Error("Failed to execute curl command", err, "command", command)
		return fmt.Errorf("failed to execute curl command '%s': %w", command, err)
	}

	log.Debug("Curl command executed successfully", "command", command)

	// Extract app name from the command
	appName := extractNameFromCurlCommand(command)
	if appName == "" {
		log.Warn("Could not determine app name from command, using 'unknown'", "command", command)
		appName = "unknown"
	}

	// Add app to repository
	if err := repo.AddApp(appName); err != nil {
		log.Error("Failed to add app to repository", err, "appName", appName)
		return fmt.Errorf("failed to add app '%s' to repository: %w", appName, err)
	}

	log.Debug("App added to repository successfully", "appName", appName)
	return nil
}

// Uninstall removes applications installed via curl pipes (not directly supported)
func (c *CurlPipeInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("CurlPipe Installer: Starting uninstallation", "command", command)

	// Extract app name from the command
	appName := extractNameFromCurlCommand(command)
	if appName == "" {
		log.Warn("Could not determine app name from command, using command as app name", "command", command)
		appName = command
	}

	// Check if the app is tracked in repository
	_, err := repo.GetApp(appName)
	if err != nil {
		log.Info("App not found in repository, skipping uninstallation", "appName", appName)
		return nil
	}

	// CurlPipe installations typically don't provide uninstall scripts
	// We can only remove from our tracking repository
	log.Warn("CurlPipe installations cannot be automatically uninstalled", "appName", appName, "hint", "Manual removal required")

	// Remove from repository tracking
	if err := repo.DeleteApp(appName); err != nil {
		log.Error("Failed to remove app from repository", err, "appName", appName)
		return fmt.Errorf("failed to remove app from repository: %w", err)
	}

	log.Debug("App removed from repository successfully", "appName", appName)
	return nil
}

// IsInstalled checks if an app installed via curl pipe is tracked in repository
func (c *CurlPipeInstaller) IsInstalled(command string) (bool, error) {
	// Extract app name from the command
	appName := extractNameFromCurlCommand(command)
	if appName == "" {
		// If we can't extract name, we can't determine installation status
		return false, fmt.Errorf("cannot determine app name from curl command: %s", command)
	}

	// For curlpipe installations, we can only check if it's tracked in our repository
	// since there's no standard way to verify curl pipe installations
	log.Debug("Checking if curlpipe app is tracked", "appName", appName)

	// This would require access to repository, but BaseInstaller interface doesn't provide it
	// For now, return false as we cannot reliably check curl pipe installations
	return false, nil
}

func extractNameFromCurlCommand(command string) string {
	parts := strings.Fields(command)
	for _, part := range parts {
		if strings.HasPrefix(part, "http") && strings.Contains(part, "/") {
			segments := strings.Split(part, "/")
			return strings.TrimSuffix(segments[len(segments)-1], ".sh")
		}
	}
	return ""
}
