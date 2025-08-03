package curlpipe

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/pkg/log"
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
