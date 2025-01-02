package curlpipe

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
)

type CurlPipeInstaller struct{}

func New() *CurlPipeInstaller {
	return &CurlPipeInstaller{}
}

func (c *CurlPipeInstaller) Install(command string, repo repository.Repository) error {
	log.Info("CurlPipe Installer: Starting installation", "command", command)

	// Run curl | sh command
	err := utilities.RunCommand(command)
	if err != nil {
		log.Error("CurlPipe Installer: Failed to execute curl command", "command", command, "error", err)
		return fmt.Errorf("failed to execute curl command: %v", err)
	}

	log.Info("CurlPipe Installer: Command executed successfully", "command", command)

	// Extract app name from the command (basic heuristic)
	appName := extractNameFromCurlCommand(command)
	if appName == "" {
		log.Warn("CurlPipe Installer: Could not determine app name from command", "command", command)
		appName = "unknown"
	}

	// Add to repository
	if err := repo.AddApp(appName); err != nil {
		log.Error("CurlPipe Installer: Failed to add app to repository", "appName", appName, "error", err)
		return fmt.Errorf("failed to add app to repository: %v", err)
	}

	log.Info("CurlPipe Installer: App added to repository", "appName", appName)
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
