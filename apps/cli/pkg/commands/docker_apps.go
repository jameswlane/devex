package commands

import (
	"embed"
	"fmt"
	"strings"

	"github.com/jameswlane/devex/pkg/installers/docker"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

//go:embed templates/docker_install.sh.tmpl
var dockerTemplateFS embed.FS

// getDockerInstallScript loads the Docker installation script template
func getDockerInstallScript() string {
	// Read the embedded template file
	scriptBytes, err := dockerTemplateFS.ReadFile("templates/docker_install.sh.tmpl")
	if err != nil {
		// Fallback to inline script if template cannot be loaded
		log.Warn("Failed to load Docker install template, using fallback", "error", err)
		return getDockerInstallScriptFallback()
	}
	return string(scriptBytes)
}

// getDockerInstallScriptFallback provides a fallback script if template loading fails
func getDockerInstallScriptFallback() string {
	return `#!/bin/bash
set -euo pipefail
echo "Error: Docker installation template could not be loaded"
echo "Please install Docker manually or check template files"
exit 1`
}

// getSelectedDatabases returns the names of selected databases
func (m *SetupModel) getSelectedDatabases() []string {
	var selected []string
	for i, isSelected := range m.selectedDBs {
		if isSelected && i < len(m.databases) {
			selected = append(selected, m.databases[i])
		}
	}
	return selected
}

// getDockerApp returns a CrossPlatformApp for Docker Engine installation
func (m *SetupModel) getDockerApp() *types.CrossPlatformApp {
	return &types.CrossPlatformApp{
		Name:        "docker",
		Description: "Container platform and runtime for developing, shipping, and running applications",
		Linux: types.OSConfig{
			InstallMethod:  "curlpipe",
			InstallCommand: getDockerInstallScript(),
		},
		MacOS: types.OSConfig{
			InstallMethod:  "brew",
			InstallCommand: "docker",
		},
		Windows: types.OSConfig{
			InstallMethod:  "winget",
			InstallCommand: "Docker.DockerDesktop",
		},
	}
}

// getDatabaseApps creates pseudo-apps for database installations via Docker
func (m *SetupModel) getDatabaseApps() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp
	selectedDBs := m.getSelectedDatabases()

	dbConfigs := map[string]map[string]string{
		"PostgreSQL": {
			"image":     "postgres:16",
			"container": "postgres16",
			"port":      docker.PostgreSQLPort,
			"env":       "POSTGRES_HOST_AUTH_METHOD=trust",
		},
		"MySQL": {
			"image":     "mysql:8.4",
			"container": "mysql8",
			"port":      docker.MySQLPort,
			"env":       "MYSQL_ALLOW_EMPTY_PASSWORD=true",
		},
		"Redis": {
			"image":     "redis:7",
			"container": "redis",
			"port":      docker.RedisPort,
			"env":       "",
		},
	}

	for _, db := range selectedDBs {
		if dbConfig, exists := dbConfigs[db]; exists {
			// Build Docker command securely with validation
			dockerCmd, err := BuildSecureDockerCommand(
				dbConfig["container"],
				dbConfig["image"],
				dbConfig["port"],
				dbConfig["env"],
			)
			if err != nil {
				log.Error("Invalid Docker configuration for database", err, "database", db)
				continue // Skip this database if configuration is invalid
			}

			app := types.CrossPlatformApp{
				Name:        fmt.Sprintf("docker-%s", strings.ToLower(db)),
				Description: fmt.Sprintf("Install %s database via Docker", db),
				Linux: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
				},
				MacOS: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
				},
				Windows: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
				},
			}
			apps = append(apps, app)
		}
	}

	return apps
}

// BuildSecureDockerCommand constructs a Docker command with proper validation and escaping
func BuildSecureDockerCommand(containerName, image, portMapping, envVar string) (string, error) {
	// Validate all inputs first
	if err := ValidateDockerConfig(containerName, image, portMapping, envVar); err != nil {
		return "", err
	}

	// Build command using safe string concatenation (not fmt.Sprintf with user input)
	var cmdParts []string
	cmdParts = append(cmdParts, "docker", "run", "-d")
	cmdParts = append(cmdParts, "--name", containerName)
	cmdParts = append(cmdParts, "--restart", "unless-stopped")
	cmdParts = append(cmdParts, "-p", "127.0.0.1:"+portMapping)

	if envVar != "" {
		cmdParts = append(cmdParts, "-e", envVar)
	}

	cmdParts = append(cmdParts, image)

	// Join with spaces - this is safe since all parts are validated
	return strings.Join(cmdParts, " "), nil
}
