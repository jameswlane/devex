package utilities

import (
	"os"
	"strings"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

func IsAppInstalled(app types.AppConfig) (bool, error) {
	log.Info("Checking if app is installed", "app", app.Name, "method", app.InstallMethod)

	commands := strings.Fields(app.InstallCommand)

	// Check each component based on the installation method
	for _, cmd := range commands {
		switch app.InstallMethod {
		case "apt":
			if !isAptInstalled(cmd) {
				log.Info("APT package not installed", "package", cmd)
				return false, nil
			}
		case "pip":
			if !isPipInstalled(cmd) {
				log.Info("PIP package not installed", "package", cmd)
				return false, nil
			}
		case "flatpak":
			if !isFlatpakInstalled(cmd) {
				log.Info("Flatpak app not installed", "appID", cmd)
				return false, nil
			}
		case "docker":
			if !isDockerInstalled(cmd) {
				log.Info("Docker container not found", "container", cmd)
				return false, nil
			}
		case "appimage":
			if !isAppImageInstalled(cmd) {
				log.Info("AppImage not found", "binary", cmd)
				return false, nil
			}
		default:
			log.Warn("Unknown install method, skipping check", "method", app.InstallMethod)
			return false, nil
		}
	}

	log.Info("All components are installed", "app", app.Name)
	return true, nil
}

func isAptInstalled(packageName string) bool {
	// First try dpkg-query with proper formatting
	command := "dpkg-query -W -f='${Status}' " + packageName
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// If dpkg-query fails, try alternative method
		log.Info("dpkg-query failed, trying alternative check", "package", packageName, "error", err)
		return isAptInstalledAlternative(packageName)
	}
	return strings.Contains(output, "install ok installed")
}

// isAptInstalledAlternative uses dpkg -l as fallback method
func isAptInstalledAlternative(packageName string) bool {
	command := "dpkg -l " + packageName
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Warn("Failed to check APT package with alternative method", "package", packageName, "error", err)
		return false
	}
	// Look for lines starting with 'ii' which indicates installed packages
	return strings.Contains(output, "ii  "+packageName)
}

func isPipInstalled(packageName string) bool {
	command := "pip show " + packageName
	_, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Warn("Failed to check PIP package", "package", packageName, "error", err)
		return false
	}
	return true
}

func isFlatpakInstalled(appID string) bool {
	command := "flatpak list --columns=application"
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Warn("Failed to check Flatpak app", "appID", appID, "error", err)
		return false
	}
	return strings.Contains(output, appID)
}

func isDockerInstalled(containerName string) bool {
	// First check if Docker daemon is running
	if !isDockerServiceAvailable() {
		log.Warn("Docker service is not available", "container", containerName)
		return false
	}

	command := "docker ps -a --format {{.Names}}"
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Warn("Failed to check Docker container", "container", containerName, "error", err)
		return false
	}
	return strings.Contains(output, containerName)
}

// isDockerServiceAvailable checks if Docker daemon is running and accessible
func isDockerServiceAvailable() bool {
	// Try to run a simple docker command to check if service is available
	_, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Server.Version}}'")
	return err == nil
}

func isAppImageInstalled(binaryPath string) bool {
	if _, err := os.Stat(binaryPath); err == nil {
		log.Info("AppImage binary found", "binaryPath", binaryPath)
		return true
	} else if os.IsNotExist(err) {
		log.Info("AppImage binary not found", "binaryPath", binaryPath)
		return false
	} else {
		log.Warn("Failed to check AppImage binary", "binaryPath", binaryPath, "error", err)
		return false
	}
}
